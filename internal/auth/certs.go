package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/names"
)

// ExtendLocalTrust makes the certs found in specified PEM string
// available as root CA certs, beyond the standard certs. It does this
// by creating an in-memory pool of certs filled from both the system
// pool and the argument, and setting this as the cert origin for
// net/http's default transport. Ditto for the websocket's default
// dialer.
func ExtendLocalTrust(certs string) {
	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	rootCAs.AppendCertsFromPEM([]byte(certs))

	// Trust the augmented cert pool in our client
	config := &tls.Config{
		RootCAs: rootCAs,
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = config
	websocket.DefaultDialer.TLSClientConfig = config

	// See https://github.com/gorilla/websocket/issues/601 for
	// what this is a work around for.
	http.DefaultTransport.(*http.Transport).ForceAttemptHTTP2 = false
}

// CertParam describes the cert-manager certificate CRD. It's passed to
// CreateCertificate to create the cert-manager certificate CR.
type CertParam struct {
	Name      string
	Namespace string
	Domain    string
	Issuer    string
}

// CreateCertificate creates a certificate resource, for the given
// cluster issuer
func CreateCertificate(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	cert CertParam,
	owner *metav1.OwnerReference,
) error {
	obj, err := newCertificate(cert)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("creation of ssl certificate for issuer '%s' failed", cert.Issuer))
	}

	client, err := cluster.ClientCertificate()
	if err != nil {
		return err
	}

	if owner != nil {
		obj.SetOwnerReferences([]metav1.OwnerReference{*owner})
	}

	_, err = client.Namespace(cert.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	// Ignore the error if it's about cert already existing.
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// newCertificate creates a proper certificate resource from the
// specified parameters. The result is suitable for upload to the
// cluster.
func newCertificate(cert CertParam) (*unstructured.Unstructured, error) {
	// Notes:
	// - spec.CommonName is length-limited.
	//   At most 64 characters are allowed, as per [RFC 3280](https://www.rfc-editor.org/rfc/rfc3280.txt).
	//   That makes it a problem for long app name and domain combinations.
	// - The spec.dnsNames (SAN, Subject Alternate Names) do not have such a limit.
	// - Luckily CN is deprecated with regard to DNS checking.
	//   The SANs are preferred and usually checked first.
	//
	// As such our solution is to
	// - Keep the full app + domain in the spec.dnsNames/SAN.
	// - Truncate the full app + domain in CN to 64 characters,
	//   replace the tail with an MD5 suffix computed over the
	//   full string as means of keeping the text unique across
	//   apps.

	cn := names.TruncateMD5(fmt.Sprintf("%s.%s", cert.Name, cert.Domain), 64)
	data := fmt.Sprintf(`{
		"apiVersion": "cert-manager.io/v1alpha2",
		"kind": "Certificate",
		"metadata": {
			"name": "%[1]s"
		},
		"spec": {
			"commonName" : "%[2]s",
			"secretName" : "%[1]s-tls",
			"dnsNames": [
				"%[1]s.%[3]s"
			],
			"issuerRef" : {
				"name" : "%[4]s",
				"kind" : "ClusterIssuer"
			}
		}
        }`, cert.Name, cn, cert.Domain, cert.Issuer)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return nil, err
	}

	return obj, err
}
