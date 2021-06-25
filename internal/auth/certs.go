// Package auth collects structures and functions around the
// generation and processing of credentials.
package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/names"
)

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

func CreateCertificate(ctx context.Context, cluster *kubernetes.Cluster, name, namespace, systemDomain string, owner *metav1.OwnerReference) error {
	// Create production certificate if systemDomain is provided by user
	// else create a local cluster self-signed tls secret.

	issuer := ""
	origin := ""

	if !strings.Contains(systemDomain, "omg.howdoi.website") {
		issuer = "letsencrypt-production"
		origin = "production"
	} else {
		issuer = "selfsigned-issuer"
		origin = "local"
	}

	obj, err := createCertificate(name, namespace, systemDomain, issuer)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("creation of %s ssl certificate failed", origin))
	}

	client, err := cluster.ClientCertificate()
	if err != nil {
		return err
	}

	if owner != nil {
		obj.SetOwnerReferences([]metav1.OwnerReference{*owner})
	}

	_, err = client.Namespace(namespace).
		Create(ctx, obj, metav1.CreateOptions{})
	// Ignore the error if it's about cert already existing.
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func createCertificate(name, namespace, systemDomain, issuer string) (*unstructured.Unstructured, error) {
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

	cn := names.TruncateMD5(fmt.Sprintf("%s.%s", name, systemDomain), 64)
	data := fmt.Sprintf(`{
		"apiVersion": "cert-manager.io/v1alpha2",
		"kind": "Certificate",
		"metadata": {
			"name": "%s",
			"namespace": "%s"
		},
		"spec": {
			"commonName" : "%s",
			"secretName" : "%s-tls",
			"dnsNames": [
				"%s.%s"
			],
			"issuerRef" : {
				"name" : "%s",
				"kind" : "ClusterIssuer"
			}
		}
        }`, name, namespace, cn, name, name, systemDomain, issuer)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return nil, err
	}

	return obj, err
}
