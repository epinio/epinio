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
	"github.com/epinio/epinio/internal/names"
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

type AuthService struct {
	typedcorev1.SecretInterface
}

func NewAuthServiceFromContext(ctx context.Context) (*AuthService, error) {
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting kubernetes cluster")
	}

	return &AuthService{
		SecretInterface: cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()),
	}, nil
}

// GetUsers returns all the Epinio users
func (s *AuthService) GetUsers(ctx context.Context) ([]User, error) {
	secrets, err := s.getUsersSecrets(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting users secrets")
	}

	users := []User{}
	for _, secret := range secrets {
		users = append(users, newUserFromSecret(secret))
	}

	return users, nil
}

// GetUserByUsername returns the user with the provided username
// It will return a UserNotFound error if the user is not found
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (User, error) {
	users, err := s.GetUsers(ctx)
	if err != nil {
		return User{}, errors.Wrap(err, "error getting users")
	}

	for _, user := range users {
		if user.Username == username {
			return user, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (s *AuthService) SaveUser(ctx context.Context, user User) (User, error) {
	userSecretName := "r" + names.GenerateResourceName("user", user.Username)

	userSecret := &corev1.Secret{
		Type: "Opaque",
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      userSecretName,
			Namespace: "epinio",
			Labels: map[string]string{
				kubernetes.EpinioAPISecretLabelKey:     "true",
				kubernetes.EpinioAPISecretRoleLabelKey: user.Role,
			},
		},
		StringData: map[string]string{
			"username": user.Username,
		},
	}

	createdUserSecret, err := s.Create(ctx, userSecret, metav1.CreateOptions{})
	if err != nil {
		return User{}, err
	}

	return newUserFromSecret(*createdUserSecret), nil
}

// AddNamespaceToUser will add the specified namespace to the User
func (s *AuthService) AddNamespaceToUser(ctx context.Context, username, namespace string) error {
	user, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return errors.Wrapf(err, "error getting user [%s] by username", username)
	}
	user.AddNamespace(namespace)

	err = s.updateUserSecret(ctx, user)
	return errors.Wrapf(err, "error updating user secret [%s]", username)
}

// RemoveNamespaceFromUsers will remove the specified namespace from all users
func (s *AuthService) RemoveNamespaceFromUsers(ctx context.Context, namespace string) error {
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

// AddGitconfigToUser will add the specified gitconfig to the User
func (s *AuthService) AddGitconfigToUser(ctx context.Context, username, gitconfig string) error {
	user, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return errors.Wrapf(err, "error getting user [%s] by username", username)
	}
	user.AddGitconfig(gitconfig)

	err = s.updateUserSecret(ctx, user)
	return errors.Wrapf(err, "error updating user secret [%s]", username)
}

// RemoveGitconfigFromUsers will remove the specified gitconfig from all users
func (s *AuthService) RemoveGitconfigFromUsers(ctx context.Context, gitconfig string) error {
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
		LabelSelector: secretSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error getting the list of the user secrets")
	}

	return secretList.Items, nil
}

func (s *AuthService) updateUserSecret(ctx context.Context, user User) error {
	// note: Wrap (nil, ...) returns nil.
	return errors.Wrap(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		userSecret, err := s.SecretInterface.Get(ctx, user.secretName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "error getting the user secret [%s]", user.Username)
		}

		userSecret.StringData = map[string]string{
			"namespaces": strings.Join(user.Namespaces, "\n"),
			"gitconfigs": strings.Join(user.Gitconfigs, "\n"),
		}

		_, err = s.SecretInterface.Update(ctx, userSecret, metav1.UpdateOptions{})
		return err
	}), fmt.Sprintf("error updating the user secret [%s]", user))
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
