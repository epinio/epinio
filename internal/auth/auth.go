// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// User is a struct containing all the information of an Epinio User
type User struct {
	Username   string
	Password   string
	CreatedAt  time.Time
	Role       string
	Namespaces []string
}

// NewUserFromSecret create an Epinio User from a Secret
func NewUserFromSecret(secret corev1.Secret) User {
	user := User{
		Username:   string(secret.Data["username"]),
		Password:   string(secret.Data["password"]),
		CreatedAt:  secret.ObjectMeta.CreationTimestamp.Time,
		Role:       secret.Labels[kubernetes.EpinioAPISecretRoleLabelKey],
		Namespaces: []string{},
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

// GetUsers returns all the Epinio users
func GetUsers(ctx context.Context) ([]User, error) {
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	secretSelector := labels.Set(map[string]string{
		kubernetes.EpinioAPISecretLabelKey: kubernetes.EpinioAPISecretLabelValue,
	}).AsSelector().String()

	// Find all user credential secrets
	secretList, err := cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()).List(ctx, metav1.ListOptions{
		FieldSelector: "type=BasicAuth",
		LabelSelector: secretSelector,
	})
	if err != nil {
		return nil, err
	}

	users := []User{}
	for _, secret := range secretList.Items {
		users = append(users, NewUserFromSecret(secret))
	}

	return users, nil
}

// GetUsersByAge returns the Epinio Users BasicAuth sorted from older to younger by CreationTime.
func GetUsersByAge(ctx context.Context) ([]User, error) {
	users, err := GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	sort.Sort(ByCreationTime(users))

	return users, nil
}
