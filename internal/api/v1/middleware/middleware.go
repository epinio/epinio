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

package middleware

import (
	"time"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/version"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func EpinioVersion(ctx *gin.Context) {
	ctx.Header(v1.VersionHeader, version.Version)
}

// InitContext initialize the Request Context injecting the logger and the requestID
func InitContext() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqCtx := ctx.Request.Context()

		requestID := uuid.NewString()

		reqCtx = requestctx.WithID(reqCtx, requestID)
		ctx.Request = ctx.Request.WithContext(reqCtx)
	}
}

// GinLogger returns a gin middleware that logs HTTP requests using zap
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if raw != "" {
			path = path + "?" + raw
		}

		logFields := []interface{}{
			"status", statusCode,
			"latency", latency,
			"clientIP", clientIP,
			"method", method,
			"path", path,
		}

		if errorMessage != "" {
			logFields = append(logFields, "error", errorMessage)
		}

		reqLogger := requestctx.Logger(c.Request.Context()).With(logFields...)

		switch {
		case statusCode >= 500:
			reqLogger.Errorw("HTTP request")
		case statusCode >= 400:
			reqLogger.Warnw("HTTP request")
		default:
			reqLogger.Infow("HTTP request")
		}
	}
}
