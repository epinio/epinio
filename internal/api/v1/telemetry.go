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

package v1

import (
	"crypto/subtle"
	"net/http"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/telemetry"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	. "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// TelemetryTriggerHeader carries the shared trigger token that authorizes a
// call to PublishTelemetry. It is not a user credential: it only proves the
// caller is the epinio-telemetry CronJob (or someone with cluster access to
// its Secret), not an end user, since this route sits behind the same public
// Ingress as the rest of the API but skips session/cookie auth.
const TelemetryTriggerHeader = "X-Epinio-Telemetry-Token" // nolint:gosec // header name, not a credential

// PublishTelemetry handles POST /api/v1/telemetry/publish. Triggered once a
// day by the epinio-telemetry CronJob (see helm-charts), it collects a fleet
// inventory snapshot and pushes it to Grafana Cloud over OTLP.
func PublishTelemetry(c *gin.Context) APIErrors {
	ctx := c.Request.Context()

	if !viper.GetBool("telemetry-enabled") {
		c.JSON(http.StatusOK, gin.H{"status": "skipped", "reason": "telemetry disabled"})
		return nil
	}

	expectedToken := viper.GetString("telemetry-trigger-token")
	providedToken := c.GetHeader(TelemetryTriggerHeader)
	if expectedToken == "" || subtle.ConstantTimeCompare([]byte(expectedToken), []byte(providedToken)) != 1 {
		return NewAPIError("invalid or missing telemetry trigger token", http.StatusUnauthorized)
	}

	cfg := telemetry.Config{
		OTLPEndpoint:      viper.GetString("telemetry-otlp-endpoint"),
		GrafanaInstanceID: viper.GetString("telemetry-grafana-instance-id"),
		GrafanaToken:      viper.GetString("telemetry-grafana-token"),
		ClusterLabel:      viper.GetString("telemetry-cluster-label"),
		EnvironmentLabel:  viper.GetString("telemetry-environment-label"),
	}

	// telemetry-enabled defaults to true (chart-side), but the Grafana Cloud
	// destination is opt-in: an install that hasn't supplied it yet is a
	// normal, expected state, not a failure the daily CronJob should report.
	if cfg.OTLPEndpoint == "" && cfg.GrafanaInstanceID == "" && cfg.GrafanaToken == "" {
		c.JSON(http.StatusOK, gin.H{"status": "skipped", "reason": "grafana cloud destination not configured"})
		return nil
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	snapshot, err := telemetry.Collect(ctx, cluster)
	if err != nil {
		return InternalError(err, "collecting telemetry snapshot")
	}

	if err := telemetry.Push(ctx, snapshot, cfg); err != nil {
		helpers.Logger.Errorw("failed to push telemetry to grafana cloud", "error", err)
		return NewInternalError("failed to publish telemetry", err.Error())
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "snapshot": snapshot})
	return nil
}
