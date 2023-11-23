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
	"bytes"
	"fmt"

	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

func Recovery(c *gin.Context) {
	stackWriter := &bytes.Buffer{}

	gin.CustomRecoveryWithWriter(stackWriter, func(c *gin.Context, anyerr any) {
		ctx := c.Request.Context()
		reqID := requestctx.ID(ctx)
		logger := requestctx.Logger(ctx).WithName("RecoveryMiddleware")

		err, ok := anyerr.(error)
		if !ok {
			err = fmt.Errorf("unknown error type occurred [%T]", anyerr)
		}

		logger.Error(err, "recovered from panic", "stack", stackWriter.String())
		fmt.Fprint(gin.DefaultWriter, stackWriter.String())

		// we don't want to expose internal details to the client
		errMsg := fmt.Sprintf("something bad happened [request ID: %s]", reqID)
		response.Error(c, apierrors.NewInternalError(errMsg))

		c.Abort()
	})(c)
}
