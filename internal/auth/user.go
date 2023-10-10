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

import (
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// User is a struct containing all the information of an Epinio User
type User struct {
	Username   string
	Password   string
	CreatedAt  time.Time
	Role       string
	Namespaces []string // list of namespaces this user has created (and thus access to)
	Gitconfigs []string // list of gitconfigs this user has created (and thus access to)

	secretName string
}

// AddNamespace adds the namespace to the User's namespaces, if it not already exists
func (u *User) AddNamespace(namespace string) {
	if namespace == "" {
		return
	}

	for _, ns := range u.Namespaces {
		if ns == namespace {
			return
		}
	}

	u.Namespaces = append(u.Namespaces, namespace)
}

// RemoveNamespace removes a namespace from the User's namespaces.
// It returns false if the namespace was not there
func (u *User) RemoveNamespace(namespace string) bool {
	updatedNamespaces := []string{}
	removed := false

	for _, ns := range u.Namespaces {
		if ns != namespace {
			updatedNamespaces = append(updatedNamespaces, ns)
		} else {
			removed = true
		}
	}

	u.Namespaces = updatedNamespaces
	return removed
}

// AddGitconfig adds the gitconfig to the User's gitconfigs, if it not already exists
func (u *User) AddGitconfig(gitconfig string) {
	if gitconfig == "" {
		return
	}

	for _, ns := range u.Gitconfigs {
		if ns == gitconfig {
			return
		}
	}

	u.Gitconfigs = append(u.Gitconfigs, gitconfig)
}

// RemoveGitconfig removes a gitconfig from the User's gitconfigs.
// It returns false if the gitconfig was not there
func (u *User) RemoveGitconfig(gitconfig string) bool {
	updatedGitconfigs := []string{}
	removed := false

	for _, ns := range u.Gitconfigs {
		if ns != gitconfig {
			updatedGitconfigs = append(updatedGitconfigs, ns)
		} else {
			removed = true
		}
	}

	u.Gitconfigs = updatedGitconfigs
	return removed
}

// newUserFromSecret create an Epinio User from a Secret
// this is an internal function that should not be used from the outside.
// It could contain internals details on how create a user from a secret.
func newUserFromSecret(secret corev1.Secret) User {
	user := User{
		Username:   string(secret.Data["username"]),
		Password:   string(secret.Data["password"]),
		CreatedAt:  secret.ObjectMeta.CreationTimestamp.Time,
		Role:       secret.Labels[kubernetes.EpinioAPISecretRoleLabelKey],
		Namespaces: []string{},
		Gitconfigs: []string{},

		secretName: secret.GetName(),
	}

	if ns, found := secret.Data["namespaces"]; found {
		namespaces := strings.TrimSpace(string(ns))
		for _, namespace := range strings.Split(namespaces, "\n") {
			namespace = strings.TrimSpace(namespace)
			if namespace != "" {
				user.Namespaces = append(user.Namespaces, namespace)
			}
		}
	}

	if gcs, found := secret.Data["gitconfigs"]; found {
		gitconfigs := strings.TrimSpace(string(gcs))
		for _, gitconfig := range strings.Split(gitconfigs, "\n") {
			gitconfig = strings.TrimSpace(gitconfig)
			if gitconfig != "" {
				user.Gitconfigs = append(user.Gitconfigs, gitconfig)
			}
		}
	}

	return user
}

// newSecretFromUser create a Secret from an Epinio User
func newSecretFromUser(user User) corev1.Secret {
	userSecretName := "r" + names.GenerateResourceName("user", user.Username)

	return corev1.Secret{
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
}
