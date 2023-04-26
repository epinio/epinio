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
	"net/http"

	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// OK reports a generic success
func OK(c *gin.Context) {
	requestctx.Logger(c.Request.Context()).Info("OK",
		"origin", c.Request.URL.String(),
		"returning", models.ResponseOK,
	)

	c.JSON(http.StatusOK, models.ResponseOK)
}

// OKBytes reports a success with some data
func OKBytes(c *gin.Context, response []byte) {
	requestctx.Logger(c.Request.Context()).Info("OK",
		"origin", c.Request.URL.String(),
		"returning", response,
	)

	c.Data(http.StatusOK, "application/octet-stream", response)
}

// OKYaml reports a success with some YAML data
func OKYaml(c *gin.Context, response interface{}) {
	requestctx.Logger(c.Request.Context()).Info("OK",
		"origin", c.Request.URL.String(),
		"returning", response,
	)

	c.YAML(http.StatusOK, response)
}

// OKReturn reports a success with some data
func OKReturn(c *gin.Context, response interface{}) {
	requestctx.Logger(c.Request.Context()).Info("OK",
		"origin", c.Request.URL.String(),
		"returning", response,
	)

	c.JSON(http.StatusOK, response)
}

// Created reports successful creation of a resource.
func Created(c *gin.Context) {
	requestctx.Logger(c.Request.Context()).Info("CREATED",
		"origin", c.Request.URL.String(),
		"returning", models.ResponseOK,
	)

	c.JSON(http.StatusCreated, models.ResponseOK)
}

// Error reports the specified errors
func Error(c *gin.Context, responseErrors errors.APIErrors) {
	requestctx.Logger(c.Request.Context()).Info("ERROR",
		"origin", c.Request.URL.String(),
		"error", responseErrors,
	)

	// add errors to the Gin context
	for _, err := range responseErrors.Errors() {
		if ginErr := c.Error(err); ginErr != nil {
			requestctx.Logger(c.Request.Context()).Error(
				ginErr, "ERROR",
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
