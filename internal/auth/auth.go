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
	ErrUserNotFound     = errors.New("user not found")
	ErrUsernameConflict = errors.New("user is defined multiple times")
)

//counterfeiter:generate -header ../../LICENSE_HEADER k8s.io/client-go/kubernetes/typed/core/v1.SecretInterface
//counterfeiter:generate -header ../../LICENSE_HEADER k8s.io/client-go/kubernetes/typed/core/v1.ConfigMapInterface

type DefinitionCount map[string]int

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

// GetUsers returns all the Epinio users with no conflicting definitions. it further returns a map
// of definition counts enabling the caller to distinguish between `truly does not exist` versus
// `has conflicting definitions`.
func (s *AuthService) GetUsers(ctx context.Context) ([]User, DefinitionCount, error) {
	s.Logger.V(1).Info("GetUsers")

	secrets, err := s.getUsersSecrets(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error getting users secrets")
	}

	// convert secrets into users and count definitions
	allUsers := []User{}
	userCount := DefinitionCount{}

	for _, secret := range secrets {
		user := newUserFromSecret(s.Logger, secret)
		allUsers = append(allUsers, user)
		userCount[user.Username] = userCount[user.Username] + 1
	}

	// remove users with conflicting definitions from the final result
	users := []User{}
	usernames := []string{} // for logging

	for _, user := range allUsers {
		if userCount[user.Username] > 1 {
			s.Logger.V(1).Info("skip duplicate user", "user", user.Username)
			continue
		}

		usernames = append(usernames, user.Username)
		users = append(users, user)
	}

	s.Logger.V(1).Info(fmt.Sprintf("found %d users", len(users)), "users", strings.Join(usernames, ","))

	// return the good users, and the map of definition counts, enablign the caller to
	// distinguish actual missing users from users weeded out because of conflicting
	// definitions.

	return users, userCount, nil
}

func (s *AuthService) GetRoles(ctx context.Context) (Roles, error) {
	s.Logger.V(1).Info("GetRoles")

	roleConfigs, err := s.getRolesConfigMaps(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting roles configmaps")
	}

	// load the Epinio roles
	epinioRoles := Roles{}
	for _, roleConfig := range roleConfigs {
		role, err := newRoleFromConfigMap(roleConfig)
		if err != nil {
			return nil, errors.Wrap(err, "loading Epinio roles")
		}
		epinioRoles = append(epinioRoles, role)
	}

	roleIDs := strings.Join(epinioRoles.IDs(), ",")
	s.Logger.V(1).Info(fmt.Sprintf("found %d roles", len(epinioRoles)), "roles", roleIDs)

	return epinioRoles, nil
}

// GetUserByUsername returns the user with the provided username
// It will return a UserNotFound error if the user is not found
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (User, error) {
	s.Logger.V(1).Info("GetUserByUsername", "username", username)

	users, counts, err := s.GetUsers(ctx)
	if err != nil {
		return User{}, errors.Wrap(err, "error getting users")
	}

	for _, user := range users {
		if user.Username == username {
			return user, nil
		}
	}

	// user not found. check if this was because it was filtered out by GetUsers() due to
	// conflicting definitions for it.

	count, ok := counts[username]
	if ok && (count > 1) {
		s.Logger.V(1).Info("user defined multiple times", "user", username, "count", count)

		return User{}, ErrUsernameConflict
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

	return newUserFromSecret(s.Logger, *createdUserSecret), nil
}

// UpdateUser will update an existing user
func (s *AuthService) UpdateUser(ctx context.Context, user User) (User, error) {
	s.Logger.V(1).Info("UpdateUser", "username", user.Username)

	userSecret, err := s.SecretInterface.Get(ctx, user.secretName, metav1.GetOptions{})
	if err != nil {
		return User{}, errors.Wrapf(err, "error getting the user secret [%s]", user.Username)
	}
	userSecret = updateUserSecretData(user, userSecret)

	updatedUserSecret, err := s.updateUserSecret(ctx, userSecret)
	if err != nil {
		return User{}, errors.Wrapf(err, "error updating user [%s]", user.Username)
	}
	updatedUser := newUserFromSecret(s.Logger, *updatedUserSecret)

	s.Logger.V(1).Info("user updated")

	return updatedUser, nil
}

// RemoveNamespaceFromUsers will remove the specified namespace from all users
func (s *AuthService) RemoveNamespaceFromUsers(ctx context.Context, namespace string) error {
	s.Logger.V(1).Info("RemoveNamespaceFromUsers", "namespace", namespace)

	users, _, err := s.GetUsers(ctx)
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

	users, _, err := s.GetUsers(ctx)
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

func (s *AuthService) updateUserSecret(ctx context.Context, userSecret *corev1.Secret) (*corev1.Secret, error) {
	var updatedSecret *corev1.Secret

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var updateErr error
		updatedSecret, updateErr = s.SecretInterface.Update(ctx, userSecret, metav1.UpdateOptions{})

		return updateErr
	})

	if err != nil {
		return nil, errors.Wrapf(err, "error updating the user secret [%s]", userSecret.Name)
	}

	return updatedSecret, nil
}

func (s *AuthService) getRolesConfigMaps(ctx context.Context) ([]corev1.ConfigMap, error) {
	configSelector := labels.Set(map[string]string{
		kubernetes.EpinioAPIConfigMapRolesLabelKey: "true",
	}).AsSelector().String()

	// Find all user credential secrets
	configList, err := s.ConfigMapInterface.List(ctx, metav1.ListOptions{
		LabelSelector: configSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error getting the list of the roles configurations")
	}

	return configList.Items, nil
}

type NamespacedResource interface {
	Namespace() string
}

// FilterResources returns only the NamespacedResources where the user has permissions
func FilterResources[T NamespacedResource](user User, resources []T) []T {
	if user.IsAdmin() {
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
	if user.IsAdmin() {
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
