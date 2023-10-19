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

// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

//counterfeiter:generate -header ../../LICENSE_HEADER k8s.io/client-go/kubernetes/typed/core/v1.SecretInterface
//counterfeiter:generate -header ../../LICENSE_HEADER k8s.io/client-go/kubernetes/typed/core/v1.ConfigMapInterface

type AuthService struct {
	Logger logr.Logger
	typedcorev1.SecretInterface
	typedcorev1.ConfigMapInterface
}

func NewAuthServiceFromContext(ctx context.Context, logger logr.Logger) (*AuthService, error) {
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting kubernetes cluster")
	}

	return NewAuthService(logger, cluster), nil
}

func NewAuthService(logger logr.Logger, cluster *kubernetes.Cluster) *AuthService {
	return &AuthService{
		Logger:             logger.WithName("AuthService"),
		SecretInterface:    cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()),
		ConfigMapInterface: cluster.Kubectl.CoreV1().ConfigMaps(helmchart.Namespace()),
	}
}

// GetUsers returns all the Epinio users
func (s *AuthService) GetUsers(ctx context.Context) ([]User, error) {
	s.Logger.V(1).Info("GetUsers")

	secrets, err := s.getUsersSecrets(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting users secrets")
	}

	users := []User{}
	usernames := []string{}
	for _, secret := range secrets {
		user := newUserFromSecret(secret)
		usernames = append(usernames, user.Username)
		users = append(users, user)
	}

	s.Logger.V(1).Info(fmt.Sprintf("found %d users", len(users)), "users", strings.Join(usernames, ","))

	return users, nil
}

// GetUserByUsername returns the user with the provided username
// It will return a UserNotFound error if the user is not found
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (User, error) {
	s.Logger.V(1).Info("GetUserByUsername", "username", username)

	users, err := s.GetUsers(ctx)
	if err != nil {
		return User{}, errors.Wrap(err, "error getting users")
	}

	for _, user := range users {
		if user.Username == username {
			return user, nil
		}
	}

	s.Logger.V(1).Info("user not found")

	return User{}, ErrUserNotFound
}

// SaveUser will save the user
func (s *AuthService) SaveUser(ctx context.Context, user User) (User, error) {
	s.Logger.V(1).Info("SaveUser", "username", user.Username)

	userSecret := newSecretFromUser(user)

	createdUserSecret, err := s.SecretInterface.Create(ctx, userSecret, metav1.CreateOptions{})
	if err != nil {
		return User{}, err
	}

	s.Logger.V(1).Info("user saved")

	return newUserFromSecret(*createdUserSecret), nil
}

// UpdateUser will update an existing user
func (s *AuthService) UpdateUser(ctx context.Context, user User) (User, error) {
	s.Logger.V(1).Info("UpdateUser", "username", user.Username)

	userSecret, err := s.SecretInterface.Get(ctx, user.secretName, metav1.GetOptions{})
	if err != nil {
		return User{}, errors.Wrapf(err, "error getting the user secret [%s]", user.Username)
	}
	userSecret = updateUserSecretData(user, userSecret)

	err = s.updateUserSecret(ctx, userSecret)
	if err != nil {
		return User{}, errors.Wrapf(err, "error updating user [%s]", user.Username)
	}

	s.Logger.V(1).Info("user updated")

	return user, nil
}

// RemoveNamespaceFromUsers will remove the specified namespace from all users
func (s *AuthService) RemoveNamespaceFromUsers(ctx context.Context, namespace string) error {
	s.Logger.V(1).Info("RemoveNamespaceFromUsers", "namespace", namespace)

	users, err := s.GetUsers(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting users")
	}

	errorMessages := []string{}
	for _, user := range users {
		removed := user.RemoveNamespace(namespace)
		// namespace was not in the Users namespaces
		if !removed {
			continue
		}

		_, err = s.UpdateUser(ctx, user)
		if err != nil {
			s.Logger.V(1).Error(err, "error removing namespace from user", "namespace", namespace, "user", user.Username)
			errorMessages = append(errorMessages, err.Error())
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("some error occurred while cleaning users: [%s]", strings.Join(errorMessages, ", "))
	}
	return nil
}

// RemoveGitconfigFromUsers will remove the specified gitconfig from all users
func (s *AuthService) RemoveGitconfigFromUsers(ctx context.Context, gitconfig string) error {
	s.Logger.V(1).Info("RemoveGitconfigFromUsers", "gitconfig", gitconfig)

	users, err := s.GetUsers(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting users")
	}

	errorMessages := []string{}
	for _, user := range users {
		removed := user.RemoveGitconfig(gitconfig)
		// gitconfig was not in the Users gitconfigs
		if !removed {
			continue
		}

		_, err = s.UpdateUser(ctx, user)
		if err != nil {
			s.Logger.V(1).Error(err, "error removing gitconfig from user", "gitconfig", gitconfig, "user", user.Username)
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
		LabelSelector: secretSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error getting the list of the user secrets")
	}

	return secretList.Items, nil
}

func (s *AuthService) updateUserSecret(ctx context.Context, userSecret *corev1.Secret) error {
	// note: Wrap (nil, ...) returns nil.
	return errors.Wrap(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := s.SecretInterface.Update(ctx, userSecret, metav1.UpdateOptions{})
		return err
	}), fmt.Sprintf("error updating the user secret [%s]", userSecret.Name))
}

type NamespacedResource interface {
	Namespace() string
}

// FilterResources returns only the NamespacedResources where the user has permissions
func FilterResources[T NamespacedResource](user User, resources []T) []T {
	if user.Role == "admin" {
		return resources
	}

	namespacesMap := make(map[string]struct{})
	for _, ns := range user.Namespaces {
		namespacesMap[ns] = struct{}{}
	}

	filteredResources := []T{}
	for _, resource := range resources {
		if _, allowed := namespacesMap[resource.Namespace()]; allowed {
			filteredResources = append(filteredResources, resource)
		}
	}

	return filteredResources
}

type GitconfigResource interface {
	Gitconfig() string
}

// FilterResources returns only the GitconfigResources where the user has permissions
func FilterGitconfigResources[T GitconfigResource](user User, resources []T) []T {
	if user.Role == "admin" {
		return resources
	}

	gitconfigsMap := make(map[string]struct{})
	for _, ns := range user.Gitconfigs {
		gitconfigsMap[ns] = struct{}{}
	}

	filteredResources := []T{}
	for _, resource := range resources {
		if _, allowed := gitconfigsMap[resource.Gitconfig()]; allowed {
			filteredResources = append(filteredResources, resource)
		}
	}

	return filteredResources
}
