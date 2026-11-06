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

package docs

//go:generate swagger generate spec

import (
	"io"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// swagger:route GET /appcharts AppCharts ChartList
// Return list of app charts.
// responses:
//   200: AppChartsResponse

// swagger:parameters ChartList
type AllChartsParam struct{}

// swagger:response AppChartsResponse
type AppChartsResponse struct {
	// in: body
	Body models.AppChartList
}

// swagger:route GET /appcharts/{Chart} AppCharts ChartShow
// Return details of the named `Chart`.
// responses:
//   200: ChartShowResponse

// swagger:parameters ChartShow
type ChartShowParam struct {
	// in: path
	Chart string
}

// swagger:response ChartShowResponse
type ChartShowResponse struct {
	// in: body
	Body models.AppChart
}

// swagger:route GET /appchartsmatch/{Pattern} AppCharts ChartMatch
// Return the chart names with prefix `Pattern`.
// responses:
//   200: ChartMatchResponse

// swagger:parameters ChartMatch
type ChartMatchParams struct {
	// in: path
	Pattern string
}

// swagger:response ChartMatchResponse
type ChartMatchResponse struct {
	// in: body
	Body models.ChartMatchResponse
}

// swagger:route GET /appchartsmatch AppCharts ChartMatch0
// Return all app chart names.
// responses:
//   200: ChartMatchResponse

// swagger:parameters ChartMatch0
type ChartMatch0Params struct {
}

// See ChartMatch above

// swagger:route POST /appcharts AppCharts ChartCreate
// Create a new appchart.
// responses:
//   201: ChartCreateResponse

// swagger:parameters ChartCreate
type ChartCreateParam struct {
	// in: body
	Body models.AppChartCreateRequest
}

// swagger:response ChartCreateResponse
type ChartCreateResponse struct{}

// swagger:route POST /appcharts/push AppCharts ChartPush
// Upload a Helm chart archive as a new app chart.
//
// The chart is pushed to Epinio's registry, and an app chart is created
// that references it.
// responses:
//   200: ChartPushResponse

// swagger:parameters ChartPush
type ChartPushParam struct {
	// The chart archive (.tgz)
	// in: formData
	// swagger:file
	File io.ReadCloser `json:"file"`
	// Name of the appchart to create
	// in: formData
	Name string `json:"name"`
	// Long description of the appchart
	// in: formData
	Description string `json:"description"`
	// Short description of the appchart
	// in: formData
	ShortDescription string `json:"short_description"`
}

// swagger:response ChartPushResponse
type ChartPushResponse struct {
	// in: body
	Body models.AppChartPushResponse
}

// swagger:route PATCH /appcharts/{Chart} AppCharts ChartUpdate
// Update fields on the named `Chart`. The chart and repository cannot be changed while applications use the `Chart`, nor for a chart pushed to Epinio's registry.
// responses:
//   200: ChartUpdateResponse

// swagger:parameters ChartUpdate
type ChartUpdateParam struct {
	// in: path
	Chart string
	// in: body
	Body models.AppChartUpdateRequest
}

// swagger:response ChartUpdateResponse
type ChartUpdateResponse struct{}

// swagger:route DELETE /appcharts/{Chart} AppCharts ChartDelete
// Delete the named `Chart`. A chart pushed to Epinio's registry is removed from it as well.
// responses:
//   200: ChartDeleteResponse

// swagger:parameters ChartDelete
type ChartDeleteParam struct {
	// in: path
	Chart string
}

// swagger:response ChartDeleteResponse
type ChartDeleteResponse struct{}
