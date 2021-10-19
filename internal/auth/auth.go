// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"k8s.io/apimachinery/pkg/labels"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PasswordAuth wraps a set of password-based credentials
type PasswordAuth struct {
	Username string
	Password string
}

// Htpasswd returns user+hash string suitable for use by Traefik's
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

func GetUserAccounts(ctx context.Context) (*gin.Accounts, error) {
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	secretSelector := labels.Set(map[string]string{
		kubernetes.EpinioAPISecretLabelKey: kubernetes.EpinioAPISecretLabelValue,
	}).AsSelector().String()

	// Find all user credential secrets
	secretList, err := cluster.Kubectl.CoreV1().Secrets("epinio").List(ctx, metav1.ListOptions{
		FieldSelector: "type=BasicAuth",
		LabelSelector: secretSelector,
	})
	if err != nil {
		return nil, err
	}

	accounts := gin.Accounts{}
	for _, secret := range secretList.Items {
		accounts[string(secret.Data["username"])] = string(secret.Data["password"])
	}

	return &accounts, nil
}
