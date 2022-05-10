// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

import (
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/gin-gonic/gin"

	corev1 "k8s.io/api/core/v1"
)

// User is a struct containing all the information of an Epinio User
type User struct {
	Username   string
	Password   string
	CreatedAt  time.Time
	Role       string
	Namespaces []string

	secretName string
}

// NewUserFromSecret create an Epinio User from a Secret
func NewUserFromSecret(secret corev1.Secret) User {
	user := User{
		Username:   string(secret.Data["username"]),
		Password:   string(secret.Data["password"]),
		CreatedAt:  secret.ObjectMeta.CreationTimestamp.Time,
		Role:       secret.Labels[kubernetes.EpinioAPISecretRoleLabelKey],
		Namespaces: []string{},

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

	return user
}

// AddNamespace adds the namespace to the User's namespaces, if not already exists
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

// MakeGinAccountsFromUsers is a utility func to convert the Epinio users to gin.Accounts,
// that can be passed to the BasicAuth middleware.
func MakeGinAccountsFromUsers(users []User) gin.Accounts {
	accounts := gin.Accounts{}
	for _, user := range users {
		accounts[user.Username] = user.Password
	}
	return accounts
}

// ByCreationTime can be used to sort Users by CreationTime
type ByCreationTime []User

func (c ByCreationTime) Len() int      { return len(c) }
func (a ByCreationTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (c ByCreationTime) Less(i, j int) bool {
	if c[i].CreatedAt == c[j].CreatedAt {
		return c[i].Username < c[j].Username
	}
	return c[i].CreatedAt.Before(c[j].CreatedAt)
}
