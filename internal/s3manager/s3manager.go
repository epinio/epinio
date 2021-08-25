// Package s3manager implements the various functions needed to store and retreive
// files from an S3 API compatible endpoint (AWS S3, Minio, etc)
package s3manager

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/google/uuid"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manager struct {
	minioClient       *minio.Client
	connectionDetails ConnectionDetails
}

type ConnectionDetails struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Location        string
}

// New returns an instance of an s3 manager
func New(connectionDetails ConnectionDetails) (*Manager, error) {
	// TODO: If we deploy our own thing, linkderd will take care of this.
	// External S3 implementations will probably needs this to be set to true.
	useSSL := false

	minioClient, err := minio.New(
		connectionDetails.Endpoint,
		&minio.Options{
			Creds:  credentials.NewStaticV4(connectionDetails.AccessKeyID, connectionDetails.SecretAccessKey, ""),
			Secure: useSSL,
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

func GetConnectionDetails(ctx context.Context, cluster *kubernetes.Cluster, secretNamespace, secretName string) (ConnectionDetails, error) {
	details := ConnectionDetails{}
	secret, err := cluster.GetSecret(ctx, secretNamespace, secretName)
	if err != nil {
		return details, err
	}

	details.Endpoint = string(secret.Data["endpoint"])
	details.AccessKeyID = string(secret.Data["access-key-id"])
	details.SecretAccessKey = string(secret.Data["secret-access-key"])
	details.Bucket = string(secret.Data["bucket"])
	details.Location = string(secret.Data["locatino"])

	return details, nil
}

func StoreConnectionDetails(ctx context.Context, cluster *kubernetes.Cluster, secretNamespace, secretName string, details ConnectionDetails) (*corev1.Secret, error) {
	secret, err := cluster.Kubectl.CoreV1().Secrets(secretNamespace).Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			StringData: map[string]string{
				"endpoint":          details.Endpoint,
				"access-key-id":     details.AccessKeyID,
				"secret-access-key": details.SecretAccessKey,
				"bucket":            details.Bucket,
				"location":          details.Location,
			},
			Type: "Opaque",
		}, metav1.CreateOptions{})

	return secret, err
}

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
	m.EnsureBucket(ctx) // TODO: Maybe we should only run this once and for all when we deploy Epinio?

	objectName := uuid.New().String()
	contentType := "application/tar"

	info, err := m.minioClient.FPutObject(ctx, m.connectionDetails.Bucket,
		objectName, filepath, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", err
	}
	// TODO: Check if there is a uid already here that can be used
	fmt.Printf("info = %+v\n", info)

	return objectName, nil
}

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
