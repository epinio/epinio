// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package s3manager implements the various functions needed to store and retrieve
// files from an S3 API compatible endpoint (AWS S3, SeaweedFS, etc)
package s3manager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/ini.v1"
)

// S3Manager defines the interface for S3 storage operations.
// This interface allows for mocking in tests.
type S3Manager interface {
	Meta(ctx context.Context, blobUID string) (map[string]string, error)
	UploadStream(ctx context.Context, file io.Reader, size int64, metadata map[string]string) (string, error)
	Upload(ctx context.Context, filepath string, metadata map[string]string) (string, error)
	EnsureBucket(ctx context.Context) error
	DeleteObject(ctx context.Context, objectID string) error
}

// Manager implements the S3Manager interface using a minio client.
type Manager struct {
	s3Client          *s3.Client
	connectionDetails ConnectionDetails
}

// Ensure Manager implements S3Manager interface
var _ S3Manager = (*Manager)(nil)

type ConnectionDetails struct {
	Endpoint        string
	UseSSL          bool
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Location        string
	CA              []byte
}

// Validate makes sure the provided S3 settings are valid
// The user should provide all the mandatory settings or no settings at all.
func (details *ConnectionDetails) Validate() error {
	allMandatorySet := details.Endpoint != "" &&
		details.AccessKeyID != "" &&
		details.SecretAccessKey != "" &&
		details.Bucket != ""
	allMandatoryEmpty := details.Endpoint == "" &&
		details.AccessKeyID == "" &&
		details.SecretAccessKey == "" &&
		details.Bucket == ""
	optionalSet := details.Location != ""

	// If mandatory fields are partly set
	partlyMandatory := !allMandatorySet && !allMandatoryEmpty
	if partlyMandatory {
		return errors.New("when specifying an external s3 server, you must set all mandatory S3 options")
	}

	// If only optional fields are set
	if allMandatoryEmpty && optionalSet {
		return errors.New("do not specify options if using the internal S3 storage")
	}

	// Either all empty or at least the mandatory fields set - valid.
	return nil
}

// New returns an instance of an s3 manager
func New(connectionDetails ConnectionDetails) (*Manager, error) {
	var httpClient *http.Client
	if len(connectionDetails.CA) > 0 {
		rootCAs := x509.NewCertPool()
		if ok := rootCAs.AppendCertsFromPEM(connectionDetails.CA); !ok {
			return nil, errors.New("cannot append S3 CA from connection details to client")
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			RootCAs:    rootCAs,
			MinVersion: tls.VersionTLS12,
		}
		httpClient = &http.Client{Transport: transport}
	}

	region := connectionDetails.Location
	if region == "" {
		region = "us-east-1"
	}

	cfg := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			connectionDetails.AccessKeyID,
			connectionDetails.SecretAccessKey,
			"",
		),
	}
	if httpClient != nil {
		cfg.HTTPClient = httpClient
	}

	// if no credentials are provided then we are going to try to connect with the IAM Role
	if connectionDetails.AccessKeyID == "" && connectionDetails.SecretAccessKey == "" {
		cfg.Credentials = nil // SDK will use default credential chain (e.g. IAM)
	}

	endpoint := connectionDetails.Endpoint
	if !strings.HasPrefix(endpoint, "http") {
		if connectionDetails.UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // required for S3-compatible backends (SeaweedFS, etc)
	})

	manager := &Manager{
		connectionDetails: connectionDetails,
		s3Client:          client,
	}

	return manager, nil
}

// GetConnectionDetails retrieves s3 details from an ini file stored in a
// secret.
// Note: The CLI tool in the staging job will use the ini file directly.
func GetConnectionDetails(ctx context.Context, cluster *kubernetes.Cluster, secretNamespace, secretName string) (ConnectionDetails, error) {
	details := ConnectionDetails{}
	secret, err := cluster.GetSecret(ctx, secretNamespace, secretName)
	if err != nil {
		return details, err
	}

	configIni, err := ini.Load(secret.Data["config"])
	if err != nil {
		return details, err
	}
	credentialsIni, err := ini.Load(secret.Data["credentials"])
	if err != nil {
		return details, err
	}

	details.AccessKeyID = credentialsIni.Section("default").Key("aws_access_key_id").MustString("")
	details.SecretAccessKey = credentialsIni.Section("default").Key("aws_secret_access_key").MustString("")
	details.Location = configIni.Section("default").Key("region").MustString("")
	details.Endpoint = string(secret.Data["endpoint"])
	if string(secret.Data["useSSL"]) == "true" {
		details.UseSSL = true
	}
	details.Bucket = string(secret.Data["bucket"])

	// load the certificate for S3 if defined
	s3CertificateSecret := viper.GetString("s3-certificate-secret")
	if s3CertificateSecret != "" {
		secret, err = cluster.GetSecret(ctx, helmchart.Namespace(), s3CertificateSecret)
		if err != nil {
			return details, errors.Wrapf(err, "getting s3 certificate secret %s", s3CertificateSecret)
		}

		details.CA = secret.Data["tls.crt"]
		if ca, ok := secret.Data["ca.crt"]; ok {
			details.CA = append(details.CA, ca...)
		}
	}

	return details, nil
}

// IsQuotaExceededError checks if an error is related to storage quota being exceeded.
// This function detects quota errors from s3gw and other S3-compatible storage backends.
func IsQuotaExceededError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Check for various quota error patterns from s3gw and other S3 backends
	return strings.Contains(errStr, "quotaexceeded") ||
		strings.Contains(errStr, "quota exceeded") ||
		strings.Contains(errStr, "quota limit") ||
		strings.Contains(errStr, "insufficient storage") ||
		strings.Contains(errStr, "storage quota") ||
		strings.Contains(errStr, "minimum free drive threshold") // Minio-specific error
}

// Meta retrieves the meta data for the blob specified by it blobUID.
func (m *Manager) Meta(ctx context.Context, blobUID string) (map[string]string, error) {
	out, err := m.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.connectionDetails.Bucket),
		Key:    aws.String(blobUID),
	})
	if err != nil {
		return map[string]string{}, errors.Wrap(err, "reading the object meta data")
	}
	if out.Metadata == nil {
		return map[string]string{}, nil
	}
	// HeadObjectOutput.Metadata is map[string]string in aws-sdk-go-v2
	meta := make(map[string]string, len(out.Metadata))
	for k, v := range out.Metadata {
		meta[k] = v
	}
	return meta, nil
}

// UploadStream uploads the given Reader to the S3 endpoint and returns a blobUID which
// can later be used to fetch the same file.
func (m *Manager) UploadStream(ctx context.Context, file io.Reader, size int64, metadata map[string]string) (string, error) {
	if err := m.EnsureBucket(ctx); err != nil {
		return "", errors.Wrap(err, "ensuring bucket")
	}

	objectName := uuid.New().String()
	contentType := "application/tar"

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(m.connectionDetails.Bucket),
		Key:         aws.String(objectName),
		Body:        file,
		ContentType: aws.String(contentType),
	}
	if size >= 0 {
		putInput.ContentLength = aws.Int64(size)
	}
	if len(metadata) > 0 {
		putInput.Metadata = make(map[string]string, len(metadata))
		for k, v := range metadata {
			putInput.Metadata[k] = v
		}
	}

	_, err := m.s3Client.PutObject(ctx, putInput)
	if err != nil {
		if IsQuotaExceededError(err) {
			return "", errors.Wrap(err, "storage quota exceeded while writing the new object")
		}
		return "", errors.Wrap(err, "writing the new object")
	}

	return objectName, nil
}

// Upload uploads the given file to the S3 endpoint and returns a blobUID which
// can later be used to fetch the same file.
func (m *Manager) Upload(ctx context.Context, filepath string, metadata map[string]string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", errors.Wrap(err, "opening file for upload")
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return "", errors.Wrap(err, "stating file")
	}

	return m.UploadStream(ctx, file, info.Size(), metadata)
}

// EnsureBucket creates our bucket if it's missing
func (m *Manager) EnsureBucket(ctx context.Context) error {
	_, err := m.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(m.connectionDetails.Bucket),
	})
	if err == nil {
		return nil
	}
	// Only create if bucket was not found
	var notFound *types.NotFound
	if !errors.As(err, &notFound) {
		return errors.Wrapf(err, "checking bucket %s exists", m.connectionDetails.Bucket)
	}
	createInput := &s3.CreateBucketInput{
		Bucket: aws.String(m.connectionDetails.Bucket),
	}
	if isAWSEndpoint(m.connectionDetails.Endpoint) &&
		m.connectionDetails.Location != "" &&
		m.connectionDetails.Location != "us-east-1" {
		createInput.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(m.connectionDetails.Location),
		}
	}
	_, createErr := m.s3Client.CreateBucket(ctx, createInput)
	if createErr != nil {
		return errors.Wrapf(createErr, "creating bucket %s", m.connectionDetails.Bucket)
	}
	return nil
}

func isAWSEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "amazonaws.com")
}

// DeleteObject deletes the specified object from the storage.
// If deletion fails due to quota errors, it returns an error that can be checked
// with IsQuotaExceededError to allow callers to handle quota issues gracefully.
func (m *Manager) DeleteObject(ctx context.Context, objectID string) error {
	_, err := m.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(m.connectionDetails.Bucket),
		Key:    aws.String(objectID),
	})
	return err
}
