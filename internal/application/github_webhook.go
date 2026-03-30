// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EpinioGitHubWebhookSecretAnnotation holds the HMAC secret GitHub uses to sign webhook payloads.
const EpinioGitHubWebhookSecretAnnotation = "epinio.io/github-webhook-secret"

// IsGitHubGitOrigin reports whether the application is sourced from GitHub (github.com or explicit provider).
func IsGitHubGitOrigin(origin models.ApplicationOrigin) bool {
	if origin.Kind != models.OriginGit || origin.Git == nil {
		return false
	}
	u, err := url.Parse(origin.Git.URL)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, "github.com") {
		return true
	}
	switch origin.Git.Provider {
	case models.ProviderGithub, models.ProviderGithubEnterprise:
		return true
	default:
		return false
	}
}

// GitHubWebhookPublicURL returns the POST URL to configure in GitHub repository webhooks.
func GitHubWebhookPublicURL(ctx context.Context, app models.AppRef) (string, error) {
	mainDomain, err := domain.MainDomain(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://epinio.%s/api/v1/webhooks/github/%s/%s", mainDomain, app.Namespace, app.Name), nil
}

// EnsureGitHubWebhookSecret generates and stores a webhook secret on the App CRD when missing.
func EnsureGitHubWebhookSecret(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef) (string, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return "", err
	}

	obj, err := client.Namespace(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	ann := obj.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	if s := ann[EpinioGitHubWebhookSecretAnnotation]; s != "" {
		return s, nil
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(secretBytes)
	ann[EpinioGitHubWebhookSecretAnnotation] = secret
	obj.SetAnnotations(ann)

	_, err = client.Namespace(app.Namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}

	return secret, nil
}

// GitHubWebhookSecretFromApp returns the stored secret, or empty if none.
func GitHubWebhookSecretFromApp(app *unstructured.Unstructured) string {
	if app == nil {
		return ""
	}
	ann := app.GetAnnotations()
	if ann == nil {
		return ""
	}
	return ann[EpinioGitHubWebhookSecretAnnotation]
}

// FillGitHubWebhookInfo sets WebhookURL and WebhookSecret on the model when the app uses GitHub.
func FillGitHubWebhookInfo(ctx context.Context, app *models.App, appCR *unstructured.Unstructured) {
	if app == nil || !IsGitHubGitOrigin(app.Origin) {
		return
	}
	u, err := GitHubWebhookPublicURL(ctx, app.Meta)
	if err != nil {
		return
	}
	app.WebhookURL = u
	app.WebhookSecret = GitHubWebhookSecretFromApp(appCR)
}
