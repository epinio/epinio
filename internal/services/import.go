// Copyright © 2026 - SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0

// External service import.
//
// An "external service" is a Helm release the user already deployed somewhere
// else on the cluster (e.g. bitnami/postgresql in a "data" namespace). Import
// makes it bindable from Epinio by:
//
//  1. Finding the release's credential secret in the source namespace.
//  2. Copying that secret into the target (Epinio) namespace, stamped with
//     the label app.kubernetes.io/instance=<ServiceReleaseName(name)> so that
//     configurations.ForService / LabelServiceSecrets find it at bind time
//     without any changes to the bind path.
//  3. Creating the Epinio tracking secret "s-<name>" so services.Get returns
//     a proper Service for list/show.

package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	epnames "github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ExternalSourceAnnotation records where an imported service came from,
	// so future sync/rotate work can find the upstream secret again.
	// Value format: "<source-namespace>/<source-secret-name>@<helm-release>".
	ExternalSourceAnnotation = "application.epinio.io/external-source"

	// ExternalCatalogValue is the CatalogServiceLabelKey value used for
	// imported services — they have no catalog entry.
	ExternalCatalogValue = "external"
)

// IsExternal reports whether a service was produced by import (vs. created
// from a catalog entry). Handles the "[Missing] " prefix that services.Get
// adds when a catalog entry can't be resolved.
func IsExternal(service *models.Service) bool {
	if service == nil {
		return false
	}
	return service.CatalogService == ExternalCatalogValue ||
		service.CatalogService == "[Missing] "+ExternalCatalogValue
}

// ImportRequest describes one import operation.
type ImportRequest struct {
	SourceNamespace string
	Release         string
	SourceSecret    string // optional override; empty = auto-pick
	TargetNamespace string
	ServiceName     string
}

// ScanReleases lists Helm releases in a namespace that the caller can see,
// grouped with the secrets each release owns. Runs with the caller's
// credentials — no privilege escalation.
func (s *ServiceClient) ScanReleases(ctx context.Context, namespace string) ([]models.ReleaseCandidate, error) {
	secrets, err := s.kubeClient.Kubectl.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=Helm",
	})
	if err != nil {
		return nil, errors.Wrap(err, "listing helm-managed secrets")
	}

	byRelease := map[string]*models.ReleaseCandidate{}
	for _, sec := range secrets.Items {
		release := sec.Labels["app.kubernetes.io/instance"]
		if release == "" {
			continue
		}
		if strings.HasPrefix(string(sec.Type), "helm.sh/release") {
			continue
		}
		c, ok := byRelease[release]
		if !ok {
			c = &models.ReleaseCandidate{
				Namespace: namespace,
				Release:   release,
				Chart:     sec.Labels["helm.sh/chart"],
			}
			byRelease[release] = c
		}
		c.Secrets = append(c.Secrets, sec.Name)
	}

	out := make([]models.ReleaseCandidate, 0, len(byRelease))
	for _, c := range byRelease {
		sort.Strings(c.Secrets)
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Release < out[j].Release })
	return out, nil
}

// ResolveReleaseSecret picks the credential secret for a Helm release.
// SourceSecret wins if set. Otherwise rank by bitnami-style naming: exact
// release name, then unique "<release>-*" match. Returns a descriptive error
// listing candidates when ambiguous.
func (s *ServiceClient) ResolveReleaseSecret(ctx context.Context, req ImportRequest) (*corev1.Secret, error) {
	if req.SourceSecret != "" {
		sec, err := s.kubeClient.GetSecret(ctx, req.SourceNamespace, req.SourceSecret)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching source secret %s/%s", req.SourceNamespace, req.SourceSecret)
		}
		return sec, nil
	}

	list, err := s.kubeClient.Kubectl.CoreV1().Secrets(req.SourceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s,app.kubernetes.io/managed-by=Helm", req.Release),
	})
	if err != nil {
		return nil, errors.Wrap(err, "listing release secrets")
	}

	var candidates []corev1.Secret
	for _, sec := range list.Items {
		if strings.HasPrefix(string(sec.Type), "helm.sh/release") {
			continue
		}
		if sec.Type == corev1.SecretTypeServiceAccountToken || sec.Type == corev1.SecretTypeTLS {
			continue
		}
		candidates = append(candidates, sec)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no credential secrets found for release %s in %s", req.Release, req.SourceNamespace)
	}
	if len(candidates) == 1 {
		return &candidates[0], nil
	}

	for _, sec := range candidates {
		if sec.Name == req.Release {
			return &sec, nil
		}
	}
	var prefixed []corev1.Secret
	for _, sec := range candidates {
		if strings.HasPrefix(sec.Name, req.Release+"-") {
			prefixed = append(prefixed, sec)
		}
	}
	if len(prefixed) == 1 {
		return &prefixed[0], nil
	}

	candidateNames := make([]string, len(candidates))
	for i, sec := range candidates {
		candidateNames[i] = sec.Name
	}
	return nil, fmt.Errorf("ambiguous credential secret for release %s: candidates=%v (pass --secret)", req.Release, candidateNames)
}

// Import copies the source secret into the target namespace with the Epinio
// service labels, and creates the tracking secret so services.Get sees it.
func (s *ServiceClient) Import(ctx context.Context, req ImportRequest) error {
	src, err := s.ResolveReleaseSecret(ctx, req)
	if err != nil {
		return err
	}

	releaseName := epnames.ServiceReleaseName(req.ServiceName)
	provenance := fmt.Sprintf("%s/%s@%s", req.SourceNamespace, src.Name, req.Release)

	// 1. Copied credential secret — only Type/Data carry over.
	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.ServiceName + "-credentials",
			Namespace: req.TargetNamespace,
			Labels: map[string]string{
				// Must equal ServiceReleaseName(req.ServiceName) — this is the
				// selector configurations.ForService uses at bind time. Any
				// deviation here and bind silently creates zero configurations.
				"app.kubernetes.io/instance":   releaseName,
				"app.kubernetes.io/managed-by": "epinio",
			},
			Annotations: map[string]string{
				ExternalSourceAnnotation: provenance,
			},
		},
		Type: src.Type,
		Data: src.Data,
	}
	if _, err := s.kubeClient.Kubectl.CoreV1().Secrets(req.TargetNamespace).Create(ctx, credSecret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "creating copied credential secret")
		}
		return fmt.Errorf("target secret %s already exists in %s", credSecret.Name, req.TargetNamespace)
	}

	// 2. Epinio tracking secret — makes services.Get / List return this service.
	trackingName := serviceResourceName(req.ServiceName)
	tracking := map[string]string{
		CatalogServiceLabelKey:        ExternalCatalogValue,
		CatalogServiceVersionLabelKey: "n-a",
		ServiceNameLabelKey:           req.ServiceName,
	}
	annotations := map[string]string{
		ExternalSourceAnnotation: provenance,
	}
	if err := s.kubeClient.CreateLabeledSecret(ctx, req.TargetNamespace, trackingName, nil, tracking, annotations); err != nil {
		_ = s.kubeClient.DeleteSecret(ctx, req.TargetNamespace, credSecret.Name)
		return errors.Wrap(err, "creating tracking secret")
	}
	return nil
}
