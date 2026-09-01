// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetry

import (
	"time"

	"github.com/epinio/epinio/internal/api/v1/response"
	fleettelemetry "github.com/epinio/epinio/internal/telemetry"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Publish collects fleet inventory and pushes it to Grafana Cloud via OTLP.
func Publish(c *gin.Context) errors.APIErrors {
	ctx := c.Request.Context()
	cfg := fleettelemetry.LoadConfig()

	if !cfg.Ready() {
		return errors.NewAPIError("fleet telemetry export is disabled or not configured", 503)
	}

	inventory, err := fleettelemetry.Collect(ctx, cfg)
	if err != nil {
		return errors.InternalError(err)
	}

	if err := fleettelemetry.Push(ctx, cfg, inventory); err != nil {
		return errors.InternalError(err)
	}

	response.OKReturn(c, models.TelemetryPublishResponse{
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
		Inventory:   inventory,
	})
	return nil
}
