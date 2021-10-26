// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"k8s.io/apimachinery/pkg/labels"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const accountExpiryInterval = 60 * time.Second

// Local globals for managing a timed cache of user accounts.
var (
	cache cacheState
)

// cacheState encapsulates the current state of the account cache.
type cacheState struct {
	Init     sync.Once     // Initializer
	Lock     sync.RWMutex  // Multi-Reader / Single-Writer Lock
	Accounts *gin.Accounts // Result of last getAccounts() call
	Err      error         // Error result of the same.
}

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

// GetUserAccounts returns all Epinio users as a gin.Accounts object to be passed to the
// BasicAuth middleware.
func GetUserAccounts(ctx context.Context) (*gin.Accounts, error) {
	// Initialize cache system
	cache.Init.Do(func() {
		// Spawn the goroutine which periodically refreshes the cache.
		go func(ctx context.Context) {
			// Operation
			// - A timed loop refreshes the cache state as per the expiry interval.
			// - This is the single writer of the system.
			// - The requestors are the reader.
			// - All use the cache.Lock to sync their access.

			for {
				current, err := getAccounts(ctx)
				func() {
					cache.Lock.Lock()
					defer cache.Lock.Unlock()

					cache.Accounts = current
					cache.Err = err
				}()
				time.Sleep(accountExpiryInterval)
			}
		}(context.Background())
	})

	cache.Lock.RLock()
	defer cache.Lock.RUnlock()

	// Wait for the goroutine to initialize the map. This may spin a bit. Should
	// affect only a few first user requests.
	if cache.Accounts == nil {
		cache.Lock.RUnlock()
		for {
			cache.Lock.RLock()
			if cache.Accounts != nil {
				break
			}
			cache.Lock.RUnlock()
		}
	}

	// Note: We are returning a pointer to a map which is used only to read from the
	// map (len, gin.BasicAuth, range loop. The locking here is only so that getting
	// the pointer in question is not troubled by the writer goroutine above.

	return cache.Accounts, cache.Err
}

// getAccounts retrieves the current set of users from the kube cluster.
func getAccounts(ctx context.Context) (*gin.Accounts, error) {
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
	secretList, err := cluster.Kubectl.CoreV1().Secrets("epinio").List(ctx, metav1.ListOptions{
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
