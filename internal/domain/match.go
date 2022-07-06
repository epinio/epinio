package domain

import (
	"context"
	"path/filepath"

	"github.com/epinio/epinio/helpers/cahash"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// domainMap is the internal type for the map from domain patterns to
// the names of the secrets holding the TLS certs serving them.
type DomainMap map[string]string

const routingSelector = "epinio.io/routing"

// MatchDo is the core matching function taking a domain and map and
// returning the secret, or nothing.
func MatchDo(domain string, domains DomainMap) (string, error) {
	if domains == nil {
		// No cert secrets, no matches possible.
		return "", nil
	}

	// Check for exact match. This has priority over any wildcards.
	if secret, ok := domains[domain]; ok {
		return secret, nil
	}

	// Pattern match for wildcard, choose longest pattern, or random at tie
	result := ""
	bestlen := 0
	for pattern, secret := range domains {
		matched, err := filepath.Match(pattern, domain)
		if err != nil {
			return "", err
		}
		if matched && len(pattern) > bestlen {
			bestlen = len(pattern)
			result = secret
		}
	}

	return result, nil
}

// MatchMapLoad queries the cluster for TLS secrets which are marked
// for use by epinio, via the routingSelector label. It returns a map
// from the domains the secrets are serving, to the serving secret.
func MatchMapLoad(ctx context.Context) DomainMap {
	listOpts := metav1.ListOptions{
		LabelSelector: routingSelector,
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil
	}

	certSecrets, err := cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()).List(ctx, listOpts)
	if err != nil {
		return nil
	}

	if len(certSecrets.Items) < 1 {
		return nil
	}

	domains := make(DomainMap)

	for _, secret := range certSecrets.Items {
		cert, err := cahash.DecodeOneCert(secret.Data["tls.crt"])
		if err != nil {
			return nil
		}

		for _, dom := range cert.DNSNames {
			domains[dom] = secret.Name
		}
	}

	return domains
}
