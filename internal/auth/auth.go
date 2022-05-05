// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"sort"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type AuthService struct {
	SecretInterface
}

//counterfeiter:generate . SecretInterface
type SecretInterface interface {
	typedcorev1.SecretInterface
}

func NewAuthServiceFromContext(ctx context.Context) (*AuthService, error) {
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	return &AuthService{
		SecretInterface: cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()),
	}, nil
}

// GetUsers returns all the Epinio users
func (s *AuthService) GetUsers(ctx context.Context) ([]User, error) {
	secrets, err := s.getUsersSecrets(ctx)
	if err != nil {
		return nil, err
	}

	users := []User{}
	for _, secret := range secrets {
		users = append(users, NewUserFromSecret(secret))
	}

	return users, nil
}

// GetUsersByAge returns the Epinio Users BasicAuth sorted from older to younger by CreationTime.
func (s *AuthService) GetUsersByAge(ctx context.Context) ([]User, error) {
	users, err := s.GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	sort.Sort(ByCreationTime(users))

	return users, nil
}

func (s *AuthService) getUsersSecrets(ctx context.Context) ([]corev1.Secret, error) {
	secretSelector := labels.Set(map[string]string{
		kubernetes.EpinioAPISecretLabelKey: kubernetes.EpinioAPISecretLabelValue,
	}).AsSelector().String()

	// Find all user credential secrets
	secretList, err := s.SecretInterface.List(ctx, metav1.ListOptions{
		FieldSelector: "type=BasicAuth",
		LabelSelector: secretSelector,
	})
	if err != nil {
		return nil, err
	}

	return secretList.Items, nil
}
