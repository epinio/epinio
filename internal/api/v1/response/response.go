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

// Package response is used by all actions to write their final result as JSON
package response

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"

	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// OK reports a generic success
func OK(c *gin.Context) {
	requestctx.Logger(c.Request.Context()).Infow("OK",
		"origin", c.Request.URL.String(),
		"returning", models.ResponseOK,
	)

	c.JSON(http.StatusOK, models.ResponseOK)
}

// OKBytes reports a success with some data
func OKBytes(c *gin.Context, response []byte) {
	requestctx.Logger(c.Request.Context()).Infow("OK",
		"origin", c.Request.URL.String(),
		"returning", response,
	)

	c.Data(http.StatusOK, "application/octet-stream", response)
}

// OKYaml reports a success with some YAML data
func OKYaml(c *gin.Context, response interface{}) {
	requestctx.Logger(c.Request.Context()).Infow("OK",
		"origin", c.Request.URL.String(),
		"returning", response,
	)

	c.YAML(http.StatusOK, response)
}

// OKReturn reports a success with some data
func OKReturn(c *gin.Context, response interface{}) {
	// SECURITY: Log only response type/summary to avoid potential secret exposure in logs.
	// The actual response is already sanitized at the endpoint level before being returned.
	requestctx.Logger(c.Request.Context()).Infow("OK",
		"origin", c.Request.URL.String(),
		"response_type", fmt.Sprintf("%T", response),
	)

	c.JSON(http.StatusOK, response)
}

// PaginatedResponse represents a generic paginated response payload.
// It wraps a slice of items with pagination metadata.
type PaginatedResponse[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

// GetPaginationParams parses optional "page" and "pageSize" query parameters.
// It returns enabled=false when neither parameter is present, so existing
// callers can keep returning the full list by default.
func GetPaginationParams(c *gin.Context, defaultPage, defaultPageSize int) (page, pageSize int, enabled bool) {
	pageStr := c.Query("page")
	pageSizeStr := c.Query("pageSize")

	if pageStr == "" && pageSizeStr == "" {
		return 0, 0, false
	}

	page = defaultPage
	pageSize = defaultPageSize
	enabled = true

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	return page, pageSize, enabled
}

// PaginateSlice applies simple page/pageSize slicing over a slice and returns
// a PaginatedResponse with metadata.
func PaginateSlice[T any](items []T, page, pageSize int) PaginatedResponse[T] {
	total := len(items)

	if pageSize <= 0 {
		pageSize = total
	}

	totalPages := 1
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	if totalPages == 0 {
		totalPages = 1
	}

	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return PaginatedResponse[T]{
		Items:      items[start:end],
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	}
}

// Created reports successful creation of a resource.
func Created(c *gin.Context) {
	requestctx.Logger(c.Request.Context()).Infow("CREATED",
		"origin", c.Request.URL.String(),
		"returning", models.ResponseOK,
	)

	c.JSON(http.StatusCreated, models.ResponseOK)
}

// Error reports the specified errors
func Error(c *gin.Context, responseErrors errors.APIErrors) {
	log := requestctx.Logger(c.Request.Context())
	log.Infow("ERROR",
		"origin", c.Request.URL.String(),
		"error", responseErrors,
	)

	// add errors to the Gin context
	for _, err := range responseErrors.Errors() {
		if ginErr := c.Error(err); ginErr != nil {
			log.Errorw("ERROR",
				"origin", c.Request.URL.String(),
				"error", ginErr,
			)
		}
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(responseErrors.FirstStatus(), errors.ErrorResponse{
		Errors: responseErrors.Errors(),
	})
}
