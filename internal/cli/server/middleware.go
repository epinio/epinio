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

// Package server provides the Epinio http server
package server

import (
	"errors"
	"time"

	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/gin-gonic/gin"
)

// Ginlogr returns a gin.HandlerFunc (middleware) that logs requests using github.com/go-logr/logr.
//
// Requests with errors are logged using logr.Error().
// Requests without errors are logged using logr.Info().
//
// It receives:
//  1. A time package format string (e.g. time.RFC3339).
//  2. A boolean stating whether to use UTC time zone or local.
//
// Note: this is a slightly modified version of https://github.com/alron/ginlogr/blob/master/logr.go
// We wanted to add more information in case of Errors
func Ginlogr(timeFormat string, utc bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		logger := requestctx.Logger(c.Request.Context())

		// some evil middlewares modify this values
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		end := time.Now()
		latency := end.Sub(start)
		if utc {
			end = end.UTC()
		}

		// todo requestUD
		commonKeyValues := []interface{}{
			"status", c.Writer.Status(),
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"ip", c.ClientIP(),
			"user-agent", c.Request.UserAgent(),
			"time", end.Format(timeFormat),
			"latency", latency,
		}

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			for _, e := range c.Errors.Errors() {
				logger.Error(errors.New(e), "error request served", commonKeyValues...)
			}
		} else {
			logger.Info("request served", commonKeyValues...)
		}
	}
}
