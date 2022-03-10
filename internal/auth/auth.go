// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

import (
	"context"
	"errors"
	"sort"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PasswordAuth wraps a set of password-based credentials
type PasswordAuth struct {
	Username string
	Password string
}

type SecretsSortable []corev1.Secret

func (a SecretsSortable) Len() int { return len(a) }
func (a SecretsSortable) Less(i, j int) bool {
	time1 := a[i].ObjectMeta.CreationTimestamp
	time2 := a[j].ObjectMeta.CreationTimestamp

	return time1.Before(&time2)
}
func (a SecretsSortable) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// Htpassword returns user+hash string suitable for use by Traefik's
// BasicAuth module.
func (auth *PasswordAuth) Htpassword() (string, error) {
	hash, err := HashBcrypt(auth.Password)
	if err != nil {
		return "", err
	}
	return auth.Username + ":" + hash, nil
}

// HashBcrypt generates an Bcrypt hash for a password.
// See https://github.com/foomo/htpasswd for the origin of this code.
// MIT licensed, as per `blob/master/LICENSE.txt`
func HashBcrypt(password string) (hash string, err error) {
	passwordBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	return string(passwordBytes), nil
}

// RandomPasswordAuth generates a random user+password
// combination. Both elements are random 16-character hex strings.
func RandomPasswordAuth() (*PasswordAuth, error) {
	user, err := randstr.Hex16()
	if err != nil {
		return nil, err
	}

	password, err := randstr.Hex16()
	if err != nil {
		return nil, err
	}

	return &PasswordAuth{
		Username: user,
		Password: password,
	}, nil
}

// GetFirstUserAccount returns the credentials of the oldest Epinio user.
// This should normally be the one created during installation unless someone
// deleted that.
func GetFirstUserAccount(ctx context.Context) (string, string, error) {
	secrets, err := GetUserSecretsByAge(ctx)
	if err != nil {
		return "", "", err
	}

	if len(secrets) > 0 {
		username := string(secrets[0].Data["username"])
		password := string(secrets[0].Data["password"])
		return username, password, nil
	} else {
		return "", "", errors.New("no user account found")
	}
}

// GetUserAccounts returns all Epinio users as a gin.Accounts object to be
// passed to the BasicAuth middleware.
func GetUserAccounts(ctx context.Context) (*gin.Accounts, error) {
	secrets, err := GetUserSecretsByAge(ctx)
	if err != nil {
		return nil, err
	}
	accounts := gin.Accounts{}
	for _, secret := range secrets {
		accounts[string(secret.Data["username"])] = string(secret.Data["password"])
	}

	return &accounts, nil
}

// GetUserSecretsByAge returns the user BasicAuth Secrets sorted from older to
// younger by creationTimestamp.
func GetUserSecretsByAge(ctx context.Context) ([]corev1.Secret, error) {
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

	// Now lets sort the list
	sort.Sort(SecretsSortable(secretList.Items))

	return secretList.Items, nil
}
