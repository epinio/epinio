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
	"net/http"

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
