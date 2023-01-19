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

package domain

import (
	"context"
	"path/filepath"

	"github.com/epinio/epinio/helpers/cahash"
	"github.com/epinio/epinio/helpers/kubernetes"

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
			// Should we simply treat this as non-match ?
			// I.e. not pass the error up, and continue ?
			// What kind of error can filepath.Match() even generate ?
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
func MatchMapLoad(ctx context.Context, namespace string) DomainMap {
	listOpts := metav1.ListOptions{
		LabelSelector: routingSelector,
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return nil
	}

	certSecrets, err := cluster.Kubectl.CoreV1().Secrets(namespace).List(ctx, listOpts)
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
