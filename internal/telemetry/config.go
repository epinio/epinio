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
	"strings"

	"github.com/spf13/viper"
)

// Config holds fleet telemetry export settings for Grafana Cloud OTLP.
type Config struct {
	Enabled     bool
	OTLPEndpoint string
	OTLPUsername string
	OTLPPassword string
	Cluster     string
	Environment string
}

// LoadConfig reads fleet telemetry settings from viper / environment.
func LoadConfig() Config {
	return Config{
		Enabled:      viper.GetBool("telemetry-enabled"),
		OTLPEndpoint: strings.TrimSpace(viper.GetString("telemetry-otlp-endpoint")),
		OTLPUsername: strings.TrimSpace(viper.GetString("telemetry-otlp-username")),
		OTLPPassword: viper.GetString("telemetry-otlp-password"),
		Cluster:      strings.TrimSpace(viper.GetString("telemetry-cluster-name")),
		Environment:  strings.TrimSpace(viper.GetString("telemetry-environment")),
	}
}

func (c Config) Ready() bool {
	return c.Enabled &&
		c.OTLPEndpoint != "" &&
		c.OTLPUsername != "" &&
		c.OTLPPassword != ""
}
