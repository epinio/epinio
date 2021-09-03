// Package s3manager implements the various functions needed to store and retrieve
// files from an S3 API compatible endpoint (AWS S3, Minio, etc)
package s3manager

import (
	"context"
	"fmt"
	"strconv"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/google/uuid"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manager struct {
	minioClient       *minio.Client
	connectionDetails ConnectionDetails
}

type ConnectionDetails struct {
	Endpoint string
	// UseSSL Toggles the use of SSL for the s3 connection. If we deploy
	// our own thing, linkderd will take care of this and we set useSSL to
	// false.
	// External S3 implementations should use SSL.
	UseSSL          bool
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Location        string
}

// New returns an instance of an s3 manager
func New(connectionDetails ConnectionDetails) (*Manager, error) {
	useSSL := connectionDetails.UseSSL

	minioClient, err := minio.New(
		connectionDetails.Endpoint,
		&minio.Options{
			Creds:  credentials.NewStaticV4(connectionDetails.AccessKeyID, connectionDetails.SecretAccessKey, ""),
			Secure: useSSL,
			Region: connectionDetails.Location,
		})
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
// Note: The CLI tool in the pipeline task will use the ini file directly.
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

	return details, nil
}

// StoreConnectionDetails stores the S3 connection details in a secret. The
// ini-file format is compatible with awscli.
// related tekton task: https://hub.tekton.dev/tekton/task/aws-cli
func StoreConnectionDetails(ctx context.Context, cluster *kubernetes.Cluster, secretNamespace, secretName string, details ConnectionDetails) (*corev1.Secret, error) {
	credentials := fmt.Sprintf(`[default]
aws_access_key_id     = %s
aws_secret_access_key = %s
`, details.AccessKeyID, details.SecretAccessKey)
	config := fmt.Sprintf(`[default]
region = %s
`, details.Location)

	secret, err := cluster.Kubectl.CoreV1().Secrets(secretNamespace).Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			StringData: map[string]string{
				"credentials": credentials,
				"config":      config,
				"endpoint":    details.Endpoint,
				"useSSL":      strconv.FormatBool(details.UseSSL),
				"bucket":      details.Bucket,
			},
			Type: "Opaque",
		}, metav1.CreateOptions{})

	return secret, err
}

// Validate makes sure all fields are set or none are set.
// TODO what happens for none, shouldn't there be defaults???
func (details *ConnectionDetails) Validate() error {
	if (details.Endpoint != "" &&
		details.AccessKeyID != "" &&
		details.SecretAccessKey != "" &&
		details.Bucket != "" &&
		details.Location != "") || (details.Endpoint == "" &&
		details.AccessKeyID == "" &&
		details.SecretAccessKey == "" &&
		details.Bucket == "" &&
		details.Location == "") {
		return nil
	}

	return errors.New("you must set all the s3 options or none")
}

// Upload uploads the given file to the S3 endpoint and returns a blobUID which
// can later be used to fetch the same file.
func (m *Manager) Upload(ctx context.Context, filepath string) (string, error) {
	if err := m.EnsureBucket(ctx); err != nil {
		return "", errors.Wrap(err, "ensuring bucket")
	}

	objectName := uuid.New().String()
	contentType := "application/tar"

	_, err := m.minioClient.FPutObject(ctx, m.connectionDetails.Bucket,
		objectName, filepath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", errors.Wrap(err, "writing the new object")
	}

	return objectName, nil
}

// EnsureBucket creates our bucket if it's missing
func (m *Manager) EnsureBucket(ctx context.Context) error {
	exists, err := m.minioClient.BucketExists(ctx, m.connectionDetails.Bucket)
	if err != nil {
		return err
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
