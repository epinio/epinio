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
	// UseSSL toggles the use of SSL for the s3 connection. If we deploy
	// our own thing, linkerd will take care of this and we set UseSSL to
	// false.
	// External S3 implementations should use SSL.
	UseSSL          bool
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Location        string
}

func NewConnectionDetails(endpoint, key, secret, bucket, location string, useSSL bool) *ConnectionDetails {
	return &ConnectionDetails{
		Endpoint:        endpoint,
		UseSSL:          useSSL,
		AccessKeyID:     key,
		SecretAccessKey: secret,
		Bucket:          bucket,
		Location:        location,
	}
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

// Meta retrieves the meta data for the blob specified by it blobUID.
func (m *Manager) Meta(ctx context.Context, blobUID string) (map[string]string, error) {
	blobInfo, err := m.minioClient.StatObject(ctx, m.connectionDetails.Bucket,
		blobUID, minio.StatObjectOptions{})
	if err != nil {
		return map[string]string{}, errors.Wrap(err, "reading the object meta data")
	}

	return blobInfo.UserMetadata, nil
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
