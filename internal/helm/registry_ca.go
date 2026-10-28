package helm

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"sync"

	hc "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/registry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/helmchart"
)

// registryCAs caches the result of registryRootCAs for the process.
var registryCAs struct {
	sync.Mutex
	loaded bool
	pool   *x509.CertPool
}

// registryRootCAs returns the CAs to verify Epinio's registry with: the system
// CAs, plus the certificate of containerregistry.certificateSecret
// (REGISTRY_CERTIFICATE_SECRET). It is the certificate the staging jobs and
// the deletion of application images trust. It returns nil without such a
// secret, for the system CAs alone.
//
// The secret is read once per process. A changed certificate takes a restart
// of the server. A failed read is not kept, and tried again next time.
func registryRootCAs(
	ctx context.Context,
	restConfig *rest.Config,
) (*x509.CertPool, error) {
	name := os.Getenv("REGISTRY_CERTIFICATE_SECRET")
	if name == "" || restConfig == nil {
		return nil, nil
	}

	registryCAs.Lock()
	defer registryCAs.Unlock()

	if registryCAs.loaded {
		return registryCAs.pool, nil
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "creating a kubernetes client")
	}

	secret, err := clientset.CoreV1().Secrets(helmchart.Namespace()).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if apierrors.IsNotFound(err) {
		helpers.Logger.Infow(
			"registry certificate secret not found",
			"secret",
			name,
		)
		registryCAs.loaded = true
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "reading the registry certificate secret")
	}

	pool, err := rootCAsWith(secret.Data["tls.crt"])
	if err != nil {
		return nil, errors.Wrapf(err, "registry certificate secret %s", name)
	}

	registryCAs.loaded = true
	registryCAs.pool = pool
	return pool, nil
}

// rootCAsWith returns the system CAs, plus the given PEM certificate.
func rootCAsWith(cert []byte) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	if !pool.AppendCertsFromPEM(cert) {
		return nil, errors.New("no certificate found in tls.crt")
	}

	return pool, nil
}

// trustRegistryCAs gives the helm client a registry client verifying
// registries with the CAs of registryRootCAs. It has to happen before the
// client is shared, as helm offers no safe way to change them later. Without a
// registry certificate the client is left as it is.
//
// A failure is logged, and the client keeps the system CAs. Only a registry
// with a private CA is affected, and it fails with a clear x509 error.
func trustRegistryCAs(
	ctx context.Context,
	restConfig *rest.Config,
	helmClient *hc.HelmClient,
) {
	pool, err := registryRootCAs(ctx, restConfig)
	if err != nil {
		helpers.Logger.Errorw(
			"loading the registry certificate, using the system CAs",
			"error",
			err,
		)
		return
	}
	if pool == nil {
		return
	}

	// Like the registry client helm-client creates, with our CAs added.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(helmClient.Settings.Debug),
		registry.ClientOptCredentialsFile(helmClient.Settings.RegistryConfig),
		registry.ClientOptHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		helpers.Logger.Errorw(
			"creating the registry client, using the system CAs",
			"error",
			err,
		)
		return
	}

	helmClient.ActionConfig.RegistryClient = registryClient
}

// tlsConfig returns the TLS configuration for talking to the registry
// directly, or nil for the defaults.
func (l *registryLogin) tlsConfig() *tls.Config {
	switch {
	case l.Insecure:
		return &tls.Config{
			InsecureSkipVerify: true, // nolint:gosec // in-cluster, self-signed
			MinVersion:         tls.VersionTLS12,
		}
	case l.RootCAs != nil:
		return &tls.Config{
			RootCAs:    l.RootCAs,
			MinVersion: tls.VersionTLS12,
		}
	}
	return nil
}
