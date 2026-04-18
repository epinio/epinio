// Copyright © 2026 - SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0

package service

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/services"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Scan handles GET /namespaces/:namespace/services/importable?source=<ns>
// The :namespace param is the target; ?source specifies where to scan.
func Scan(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).With("component", "ServiceScan")

	source := c.Query("source")
	if source == "" {
		return apierror.NewBadRequestError("missing 'source' query param")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	client, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	candidates, err := client.ScanReleases(ctx, source)
	if err != nil {
		logger.Errorw("scan failed", "error", err)
		return apierror.InternalError(err)
	}

	response.OKReturn(c, models.ServiceImportScanResponse{Candidates: candidates})
	return nil
}

// Import handles POST /namespaces/:namespace/services/import
func Import(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).With("component", "ServiceImport")

	targetNamespace := c.Param("namespace")

	var req models.ServiceImportRequest
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequestError(err.Error())
	}
	if req.SourceNamespace == "" || req.Release == "" || req.ServiceName == "" {
		return apierror.NewBadRequestError("source_namespace, release, and service_name are required")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	client, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = client.Import(ctx, services.ImportRequest{
		SourceNamespace: req.SourceNamespace,
		Release:         req.Release,
		SourceSecret:    req.SourceSecret,
		TargetNamespace: targetNamespace,
		ServiceName:     req.ServiceName,
	})
	if err != nil {
		logger.Errorw("import failed", "error", err)
		return apierror.InternalError(err)
	}

	response.Created(c)
	return nil
}
