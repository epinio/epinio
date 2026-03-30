// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const githubWebhookUser = "github-webhook"

// GitHubWebhook handles POST /api/v1/webhooks/github/:namespace/:app (no API auth; HMAC verified).
func GitHubWebhook(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	appName := c.Param("app")

	body, err := c.GetRawData()
	if err != nil {
		return apierror.NewBadRequestError("reading body")
	}

	event := c.GetHeader("X-GitHub-Event")
	if event == "ping" {
		c.JSON(http.StatusOK, models.ResponseOK)
		return nil
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	appRef := models.NewAppRef(appName, namespace)
	appCR, err := application.Get(ctx, cluster, appRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown(appName)
		}
		return apierror.InternalError(err, "failed to get the application resource")
	}

	secret := application.GitHubWebhookSecretFromApp(appCR)
	if secret == "" {
		return apierror.NewBadRequestError("webhook secret not configured for this application; deploy the app from GitHub first")
	}

	sig := c.GetHeader("X-Hub-Signature-256")
	if !verifyGitHubSignature256([]byte(secret), body, sig) {
		return apierror.NewAPIError("invalid webhook signature", http.StatusUnauthorized)
	}

	if event != "push" {
		c.JSON(http.StatusOK, models.ResponseOK)
		return nil
	}

	var payload struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Deleted    bool   `json:"deleted"`
		Repository *struct {
			CloneURL string `json:"clone_url"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return apierror.NewBadRequestError("invalid JSON payload")
	}

	if payload.Deleted || payload.After == "" || strings.HasPrefix(payload.After, "0000000") {
		c.JSON(http.StatusOK, models.ResponseOK)
		return nil
	}

	origin, err := application.Origin(appCR)
	if err != nil {
		return apierror.InternalError(err, "reading app origin")
	}
	if origin.Kind != models.OriginGit || origin.Git == nil {
		return apierror.NewBadRequestError("application is not deployed from git")
	}
	if !application.IsGitHubGitOrigin(origin) {
		return apierror.NewBadRequestError("GitHub webhooks are only supported for GitHub-sourced applications")
	}

	if payload.Repository == nil || payload.Repository.CloneURL == "" {
		return apierror.NewBadRequestError("missing repository clone_url in payload")
	}
	if !gitHubReposMatch(origin.Git.URL, payload.Repository.CloneURL) {
		log.Infow("github webhook ignored: repository mismatch",
			"app", appName, "expected", origin.Git.URL, "got", payload.Repository.CloneURL)
		c.JSON(http.StatusOK, models.ResponseOK)
		return nil
	}

	pushBranch := refToBranchName(payload.Ref)
	if origin.Git.Branch != "" && pushBranch != "" && !strings.EqualFold(origin.Git.Branch, pushBranch) {
		log.Infow("github webhook ignored: branch mismatch",
			"app", appName, "tracked", origin.Git.Branch, "push", pushBranch)
		c.JSON(http.StatusOK, models.ResponseOK)
		return nil
	}

	staging, err := application.IsCurrentlyStaging(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if staging {
		return apierror.NewBadRequestError("application is already staging")
	}

	c.JSON(http.StatusAccepted, models.ResponseOK)

	go runGitHubPushRebuild(context.Background(), appRef, origin, payload.After, pushBranch)

	return nil
}

func runGitHubPushRebuild(ctx context.Context, appRef models.AppRef, origin models.ApplicationOrigin, revision, branchHint string) {
	log := helpers.Logger
	if log == nil {
		return
	}
	log.Infow("github webhook rebuild started", "app", appRef.Name, "namespace", appRef.Namespace, "revision", revision)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		log.Errorw("github webhook rebuild: cluster", "error", err)
		return
	}

	appCR, err := application.Get(ctx, cluster, appRef)
	if err != nil {
		log.Errorw("github webhook rebuild: get app", "error", err)
		return
	}

	gitURL := origin.Git.URL
	importResp, apiErr := ImportGitRun(ctx, cluster, appRef.Namespace, appRef.Name, gitURL, revision, webhookActorUsername(appCR))
	if apiErr != nil {
		log.Errorw("github webhook rebuild: import git", "errors", apiErr)
		return
	}

	stageReq := models.StageRequest{
		App:     appRef,
		BlobUID: importResp.BlobUID,
	}

	stageResp, apiErr := StageRun(ctx, cluster, stageReq, webhookActorUsername(appCR), appCR, appRef.Namespace)
	if apiErr != nil {
		log.Errorw("github webhook rebuild: stage", "errors", apiErr)
		return
	}

	jobs, apiErr := stageJobs(ctx, cluster, appRef.Namespace, stageResp.Stage.ID)
	if apiErr != nil {
		log.Errorw("github webhook rebuild: stage jobs", "errors", apiErr)
		return
	}

	ok, err := waitForStagingCompletion(ctx, cluster, jobs)
	if err != nil {
		log.Errorw("github webhook rebuild: wait staging", "error", err)
		return
	}
	if !ok {
		log.Errorw("github webhook rebuild: staging failed", "stageID", stageResp.Stage.ID)
		return
	}

	appCR, err = application.Get(ctx, cluster, appRef)
	if err != nil {
		log.Errorw("github webhook rebuild: refresh app", "error", err)
		return
	}

	err = deploy.UpdateImageURL(ctx, cluster, appCR, stageResp.ImageURL)
	if err != nil {
		log.Errorw("github webhook rebuild: update image url", "error", err)
		return
	}

	newOrigin := origin
	newOrigin.Git.Revision = importResp.Revision
	if importResp.Branch != "" {
		newOrigin.Git.Branch = importResp.Branch
	} else if branchHint != "" {
		newOrigin.Git.Branch = branchHint
	}

	deployResult, apiErr := deploy.DeployApp(ctx, cluster, appRef, webhookActorUsername(appCR), stageResp.Stage.ID)
	if apiErr != nil {
		log.Errorw("github webhook rebuild: deploy", "errors", apiErr)
		return
	}

	err = application.SetOrigin(ctx, cluster, appRef, newOrigin)
	if err != nil {
		log.Errorw("github webhook rebuild: set origin", "error", err)
		return
	}

	log.Infow("github webhook rebuild completed", "app", appRef.Name, "routes", deployResult.Routes)
}

func webhookActorUsername(appCR *unstructured.Unstructured) string {
	if appCR == nil {
		return githubWebhookUser
	}
	ann := appCR.GetAnnotations()
	if ann == nil {
		return githubWebhookUser
	}
	if u := ann[models.EpinioCreatedByAnnotation]; u != "" {
		return u
	}
	return githubWebhookUser
}

func verifyGitHubSignature256(secret, body []byte, signature string) bool {
	if signature == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	got := strings.TrimPrefix(signature, prefix)
	return hmac.Equal([]byte(expected), []byte(got))
}

func refToBranchName(ref string) string {
	const p = "refs/heads/"
	if strings.HasPrefix(ref, p) {
		return strings.TrimPrefix(ref, p)
	}
	return ""
}

func gitHubReposMatch(appURL, payloadURL string) bool {
	return normalizeGitHubRepoURL(appURL) == normalizeGitHubRepoURL(payloadURL)
}

func normalizeGitHubRepoURL(raw string) string {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, ".git"))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "git@") {
		rest := strings.TrimPrefix(raw, "git@")
		idx := strings.Index(rest, ":")
		if idx < 0 {
			return strings.ToLower(rest)
		}
		host := rest[:idx]
		path := rest[idx+1:]
		return strings.ToLower(host + "/" + path)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(raw)
	}
	path := strings.TrimPrefix(u.Path, "/")
	return strings.ToLower(u.Host + "/" + path)
}
