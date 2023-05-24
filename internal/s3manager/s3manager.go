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
// files from an S3 API compatible endpoint (AWS S3, Minio, etc)
package s3manager

import (
	"context"
	"crypto/x509"
	"io"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/google/uuid"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/ini.v1"
)

type Manager struct {
	minioClient       *minio.Client
	connectionDetails ConnectionDetails
}

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
	partlyMandatory := !(allMandatorySet || allMandatoryEmpty)
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
	transport := http.DefaultTransport.(*http.Transport).Clone()

	opts := &minio.Options{
		Creds:     credentials.NewStaticV4(connectionDetails.AccessKeyID, connectionDetails.SecretAccessKey, ""),
		Secure:    connectionDetails.UseSSL,
		Transport: transport,
		Region:    connectionDetails.Location,
	}

	// if no credentials are provided then we are going to try to connect with the IAM Role
	if connectionDetails.AccessKeyID == "" && connectionDetails.SecretAccessKey == "" {
		opts.Creds = credentials.NewIAM("")
	}

	if len(connectionDetails.CA) > 0 {
		rootCAs := x509.NewCertPool()
		if ok := rootCAs.AppendCertsFromPEM(connectionDetails.CA); !ok {
			return nil, errors.New("cannot append minio ca from connection details to client")
		}

		tlsConfig := transport.TLSClientConfig.Clone()
		tlsConfig.RootCAs = rootCAs

		opts.Transport.(*http.Transport).TLSClientConfig = tlsConfig
	}

	minioClient, err := minio.New(
		connectionDetails.Endpoint,
		opts,
	)
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		connectionDetails: connectionDetails,
		minioClient:       minioClient,
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

// Meta retrieves the meta data for the blob specified by it blobUID.
func (m *Manager) Meta(ctx context.Context, blobUID string) (map[string]string, error) {
	blobInfo, err := m.minioClient.StatObject(ctx, m.connectionDetails.Bucket,
		blobUID, minio.StatObjectOptions{})
	if err != nil {
		return map[string]string{}, errors.Wrap(err, "reading the object meta data")
	}

	return blobInfo.UserMetadata, nil
}

// UploadStream uploads the given Reader to the S3 endpoint and returns a blobUID which
// can later be used to fetch the same file.
func (m *Manager) UploadStream(ctx context.Context, file io.Reader, size int64, metadata map[string]string) (string, error) {
	if err := m.EnsureBucket(ctx); err != nil {
		return "", errors.Wrap(err, "ensuring bucket")
	}

	objectName := uuid.New().String()
	contentType := "application/tar"

	_, err := m.minioClient.PutObject(ctx, m.connectionDetails.Bucket,
		objectName, file, size, minio.PutObjectOptions{
			ContentType:  contentType,
			UserMetadata: metadata,
		})
	if err != nil {
		return "", errors.Wrap(err, "writing the new object")
	}

	return objectName, nil
}

// Upload uploads the given file to the S3 endpoint and returns a blobUID which
// can later be used to fetch the same file.
func (m *Manager) Upload(ctx context.Context, filepath string, metadata map[string]string) (string, error) {
	if err := m.EnsureBucket(ctx); err != nil {
		return "", errors.Wrap(err, "ensuring bucket")
	}

	objectName := uuid.New().String()
	contentType := "application/tar"

	_, err := m.minioClient.FPutObject(ctx, m.connectionDetails.Bucket,
		objectName, filepath, minio.PutObjectOptions{
			ContentType:  contentType,
			UserMetadata: metadata,
		})
	if err != nil {
		return "", errors.Wrap(err, "writing the new object")
	}

	return objectName, nil
}

// EnsureBucket creates our bucket if it's missing
func (m *Manager) EnsureBucket(ctx context.Context) error {
	exists, err := m.minioClient.BucketExists(ctx, m.connectionDetails.Bucket)
	if err != nil {
		return errors.Wrapf(err, "checking bucket %s exists", m.connectionDetails.Bucket)
	}
	if exists {
		return nil
	}

	return m.minioClient.MakeBucket(ctx, m.connectionDetails.Bucket,
		minio.MakeBucketOptions{Region: m.connectionDetails.Location})
}

// DeleteObject deletes the specified object from the storage
func (m *Manager) DeleteObject(ctx context.Context, objectID string) error {
	return m.minioClient.RemoveObject(ctx, m.connectionDetails.Bucket, objectID,
		minio.RemoveObjectOptions{})
}
