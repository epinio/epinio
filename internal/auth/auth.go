// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

//counterfeiter:generate . SecretInterface
type SecretInterface interface {
	typedcorev1.SecretInterface
}

type AuthService struct {
	SecretInterface
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

// GetUserByUsername return the user with the provided username
// It will return a UserNotFound error if the user is not found
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (User, error) {
	users, err := s.GetUsers(ctx)
	if err != nil {
		return User{}, err
	}

	for _, user := range users {
		if user.Username == username {
			return user, nil
		}
	}
	return User{}, ErrUserNotFound
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

func (s *AuthService) AddNamespaceToUser(ctx context.Context, username, namespace string) error {
	user, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	user.AddNamespace(namespace)

	err = s.updateUserSecret(ctx, user)
	return err
}

func (s *AuthService) RemoveNamespaceFromUsers(ctx context.Context, namespace string) error {
	users, err := s.GetUsers(ctx)
	if err != nil {
		return err
	}

	errorMessages := []string{}
	for _, user := range users {
		user.RemoveNamespace(namespace)
		err = s.updateUserSecret(ctx, user)
		if err != nil {
			errorMessages = append(errorMessages, err.Error())
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("some error occurred while cleaning users: [%s]", strings.Join(errorMessages, ", "))
	}
	return nil
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

func (s *AuthService) updateUserSecret(ctx context.Context, user User) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		userSecret, err := s.SecretInterface.Get(ctx, user.secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if len(user.Namespaces) > 0 {
			userSecret.StringData = map[string]string{
				"namespaces": strings.Join(user.Namespaces, "\n"),
			}
		}

		_, err = s.SecretInterface.Update(ctx, userSecret, metav1.UpdateOptions{})
		return err
	})
}
