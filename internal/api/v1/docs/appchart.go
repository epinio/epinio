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

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// swagger:route GET /appcharts appcharts AllCharts
// Return list of app charts.
// responses:
//   200: AppChartsResponse

// swagger:parameters AllCharts
type AllChartsParam struct{}

// swagger:response AppChartsResponse
type AppChartsResponse struct {
	// in: body
	Body models.AppChartList
}

// swagger:route GET /appcharts/{Chart} appcharts ChartShow
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

// swagger:route GET /appchartsmatch/{Pattern} appcharts ChartMatch
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

// swagger:route GET /appchartsmatch appcharts ChartMatch0
// Return the chart names. (No prefix == Empty prefix == All match)
// responses:
//   200: ChartMatchResponse

// swagger:parameters ChartMatch0
type ChartMatch0Params struct {
}

// See ChartMatch above
