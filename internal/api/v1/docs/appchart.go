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

// swagger:route POST /appcharts appcharts ChartCreate
// Create the posted new chart.
// responses:
//   200: ChartCreateResponse

// swagger:parameters ChartCreate
type ChartCreateParam struct {
	// in: body
	Body models.ChartCreateRequest
}

// swagger:response ChartCreateResponse
type ChartCreateResponse struct {
	// in: body
	Body models.Response
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

// swagger:route DELETE /applcharts/{Chart} appcharts ChartDelete
// Delete the named `Chart`.
// responses:
//   200: ChartDeleteResponse

// swagger:parameters ChartDelete
type ChartDeleteParam struct {
	// in: path
	Chart string
}

// swagger:response ChartDeleteResponse
type ChartDeleteResponse struct {
	// in: body
	Body models.Response
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
