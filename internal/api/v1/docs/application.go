// Copyright © 2021 - 2026 KRUMWARE LLC
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

// swagger:route GET /applications Applications AllApps
// Return list of applications in all namespaces.
// Never-deployed apps (buildstatus create|build) are omitted; use AllAppsUndeployed for those.
// responses:
//   200: AppsResponse

// swagger:parameters AllApps
type AllAppsParam struct {
	// Comma-separated namespaces to filter by, e.g. "ns1,ns2". Names the user
	// cannot see, or that do not exist, are ignored.
	// in: query
	Namespaces string `json:"namespaces"`
	// Case-insensitive substring match on the application name
	// in: query
	Search string `json:"search"`
	// Page number, 1-based. Pagination is off unless page or pageSize is set.
	// in: query
	Page string `json:"page"`
	// Items per page (default 25)
	// in: query
	PageSize string `json:"pageSize"`
}

// response: See Apps.

// swagger:route GET /applications/undeployed Applications AllAppsUndeployed
// Return the applications not deployed yet, in all namespaces.
//
// Lists apps with build status `create` or `build`, which includes apps
// rebuilt after a deploy.
// responses:
//   200: AppsResponse

// swagger:parameters AllAppsUndeployed
type AllAppsUndeployedParam struct {
	// Comma-separated namespaces to filter by, e.g. "ns1,ns2". Names the user
	// cannot see, or that do not exist, are ignored.
	// in: query
	Namespaces string `json:"namespaces"`
	// Case-insensitive substring match on the application name
	// in: query
	Search string `json:"search"`
	// Page number, 1-based. Pagination is off unless page or pageSize is set.
	// in: query
	Page string `json:"page"`
	// Items per page (default 25)
	// in: query
	PageSize string `json:"pageSize"`
}

// swagger:route GET /namespaces/{Namespace}/applications Applications Apps
// Return list of applications in the `Namespace`.
// Never-deployed apps (buildstatus create|build) are omitted; use AppsUndeployed for those.
// responses:
//   200: AppsResponse

// swagger:parameters Apps
type AppsParam struct {
	// in: path
	Namespace string
	// Case-insensitive substring match on the application name
	// in: query
	Search string `json:"search"`
	// Page number, 1-based. Pagination is off unless page or pageSize is set.
	// in: query
	Page string `json:"page"`
	// Items per page (default 25)
	// in: query
	PageSize string `json:"pageSize"`
}

// swagger:route GET /namespaces/{Namespace}/applications/undeployed Applications AppsUndeployed
// Return the applications not deployed yet, in the `Namespace`.
//
// Lists apps with build status `create` or `build`, which includes apps
// rebuilt after a deploy.
// responses:
//   200: AppsResponse

// swagger:parameters AppsUndeployed
type AppsUndeployedParam struct {
	// in: path
	Namespace string
	// Case-insensitive substring match on the application name
	// in: query
	Search string `json:"search"`
	// Page number, 1-based. Pagination is off unless page or pageSize is set.
	// in: query
	Page string `json:"page"`
	// Items per page (default 25)
	// in: query
	PageSize string `json:"pageSize"`
}

// swagger:response AppsResponse
type AppsResponse struct {
	// in: body
	Body models.AppList
}

// swagger:route POST /namespaces/{Namespace}/applications Applications AppCreate
// Create the posted new application in the `Namespace`.
// responses:
//   200: AppCreateResponse

// swagger:parameters AppCreate
type AppCreateParam struct {
	// in: path
	Namespace string
	// in: body
	Configuration models.ApplicationCreateRequest
}

// swagger:response AppCreateResponse
type AppCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{Namespace}/applications/{App} Applications AppShow
// Return details of the named `App` in the `Namespace`.
// Works for both deployed and never-deployed (create|build) apps.
// The response includes `configuration.environment` (user-set variables only) and
// `configuration.environment_grouped` (user and service-provided variables, same structure as GET .../environment?grouped=true).
// responses:
//   200: AppShowResponse

// swagger:parameters AppShow
type AppShowParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppShowResponse
type AppShowResponse struct {
	// in: body
	Body models.App
}

// swagger:route GET /namespaces/{Namespace}/appsmatches/{Pattern} Applications AppMatch
// Set includeUndeployed=true to also match never-deployed (undeployed) apps.
// Return list of names for all applications whose name matches the prefix `Pattern`.
// responses:
//   200: AppMatchResponse

// swagger:parameters AppMatch
type AppMatchParam struct {
	// in: path
	Namespace string
	// in: path
	Pattern string
}

// swagger:response AppMatchResponse
type AppMatchResponse struct {
	// in: body
	Body models.AppMatchResponse
}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/part/{Part} Applications AppPart
// Return parts of the named `App` in the `Namespace`.
// responses:
//   200: AppPartResponse

// swagger:parameters AppPart
type AppPartParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: path
	Part string
}

// swagger:response AppPartResponse
type AppPartResponse struct {
	// in: body
	Body []byte
}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/logs Applications AppLogs
// Return logs of the named `App` in the `Namespace` streamed over a websocket.
// responses:
//   200: AppLogsResponse

// swagger:parameters AppLogs
type AppLogsParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// Stream logs in real-time (true/false)
	// in: query
	Follow string `json:"follow"`
	// Limit to last N lines
	// in: query
	Tail string `json:"tail"`
	// Show logs from duration ago (e.g. "1h", "30m")
	// in: query
	Since string `json:"since"`
	// Show logs since RFC3339 timestamp
	// in: query
	SinceTime string `json:"since_time"`
	// Comma-separated container names/patterns to include
	// in: query
	IncludeContainers string `json:"include_containers"`
	// Comma-separated container names/patterns to exclude
	// in: query
	ExcludeContainers string `json:"exclude_containers"`
}

// swagger:response AppLogsResponse
type AppLogsResponse struct{}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/exec Applications AppExec
// Get a shell to the `App` in the `Namespace`.
// responses:
//   200: AppExecResponse

// swagger:parameters AppExec
type AppExecParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: query
	Instance string
}

// swagger:response AppExecResponse
type AppExecResponse struct{}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/portforward Applications AppPortForward
// Get a shell to the `App` in the `Namespace`.
// responses:
//   200: AppPortForwardResponse

// swagger:parameters AppPortForward
type AppPortForwardParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: query
	Instance string
}

// swagger:response AppPortForwardResponse
type AppPortForwardResponse struct{}

// swagger:route GET /namespaces/{Namespace}/staging/{StageID}/logs Applications StagingLogs
// Return logs of the named `StageID` in the `Namespace` streamed over a websocket.
// responses:
//   200: StagingLogsResponse

// swagger:parameters StagingLogs
type StagingLogsParam struct {
	// in: path
	Namespace string
	// in: path
	StageID string
	// Stream logs in real-time (true/false)
	// in: query
	Follow string `json:"follow"`
	// Limit to last N lines
	// in: query
	Tail string `json:"tail"`
	// Show logs from duration ago (e.g. "1h", "30m")
	// in: query
	Since string `json:"since"`
	// Show logs since RFC3339 timestamp
	// in: query
	SinceTime string `json:"since_time"`
	// Comma-separated container names/patterns to include
	// in: query
	IncludeContainers string `json:"include_containers"`
	// Comma-separated container names/patterns to exclude
	// in: query
	ExcludeContainers string `json:"exclude_containers"`
}

// swagger:response StagingLogsResponse
type StagingLogsResponse struct{}

// swagger:route DELETE /namespaces/{Namespace}/applications Applications AppBatchDelete
// Delete the named `Applications` in the `Namespace`.
// responses:
//   200: AppDeleteResponse

// swagger:route DELETE /namespaces/{Namespace}/applications/{App} Applications AppDelete
// Delete the named `App` in the `Namespace`.
// responses:
//   200: AppDeleteResponse

// swagger:parameters AppDelete
type AppDeleteParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:route DELETE /namespaces/{Namespace}/applications/{App}/build Applications AppBuildDelete
// Delete the current build of the named `App` in the `Namespace` without deleting the app.
// responses:
//   200: AppBuildDeleteResponse

// swagger:parameters AppBuildDelete
type AppBuildDeleteParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppBuildDeleteResponse
type AppBuildDeleteResponse struct {
	// in: body
	Body models.Response
}

// swagger:parameters AppBatchDelete
type AppBatchDeleteParam struct {
	// in: path
	Namespace string
	// in: url
	Applications []string
}

// swagger:response AppDeleteResponse
type AppDeleteResponse struct {
	// in: body
	Body models.ApplicationDeleteResponse
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/store Applications AppUpload
// Store the named `App` in the `Namespace`.
// responses:
//   200: AppUploadResponse

// swagger:parameters AppUpload
type AppUploadParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppUploadResponse
type AppUploadResponse struct {
	// in: body
	Body models.UploadResponse
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/restart Applications AppRestart
// Restart the named `App` in the `Namespace`.
//
// Restarts the deployed image; it never switches to a newer build. Deploy a
// build with AppDeploy. Starts a restart operation and answers 202 with it.
// Poll AppState until the operation is done.
// responses:
//   202: AppOperationResponse
//   400: ErrorResponse
//   409: ErrorResponse

// swagger:parameters AppRestart
type AppRestartParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/stage Applications AppStage
// Build an image for the named `App` in the `Namespace`.
//
// Starts a build operation and answers 202 with it. The source is the
// uploaded `blobuid`, else the stored blob, else the git origin, which the
// server clones and stamps with the resolved branch and commit. Poll
// AppState until the operation is done; its `stage_id` names the build
// for the staging logs. The build does not deploy.
// responses:
//   202: AppOperationResponse
//   400: ErrorResponse
//   409: ErrorResponse

// swagger:parameters AppStage
type AppStageParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.StageRequest
}

// swagger:response AppOperationResponse
type AppOperationResponse struct {
	// in: body
	Body models.AppOperation
}

// swagger:response OperationsResponse
type OperationsResponse struct {
	// in: body
	Body models.OperationsResponse
}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/state Applications AppState
// Return the lifecycle state of the named `App` in the `Namespace`.
//
// One call for status, build status, current stage and the latest
// operation. While an operation stages or deploys, `status` reports that
// phase. `operation` is absent after a server restart or once the record
// of a finished operation expired.
// responses:
//   200: AppStateResponse

// swagger:parameters AppState
type AppStateParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppStateResponse
type AppStateResponse struct {
	// in: body
	Body models.AppState
}

// swagger:route PATCH /namespaces/{Namespace}/applications/{App}/source Applications AppSourcePatch
// Replace the source of the named `App` in the `Namespace` and rebuild it.
//
// The multipart body carries the source archive and an optional
// `process_cmd` form field (supervisor fallback command for non-Paketo
// buildpacks). Answers 202 with an operation that already carries the
// `stage_id`; it builds and swaps the new image into the running pod.
// responses:
//   202: AppOperationResponse
//   409: ErrorResponse

// swagger:parameters AppSourcePatch
type AppSourcePatchParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/sync Applications AppSync
// Sync files or a binary into the running pod of the `App`.
//
// The multipart body carries the tar archive plus `mode` ("files" or
// "binary"), and optional `dest` and `binary_name` form fields.
// responses:
//   200: AppSyncResponse

// swagger:parameters AppSync
type AppSyncParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppSyncResponse
type AppSyncResponse struct {
	// in: body
	Body models.Response
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/deploy Applications AppDeploy
// Deploy the named `App` in the `Namespace`.
//
// Starts a deploy operation and answers 202 with it. With `image` set it
// deploys that image (`stage` names the build that produced it). Without
// it, it builds first, from the same sources AppStage accepts, then
// deploys the result. Poll AppState until the operation is done.
// responses:
//   202: AppOperationResponse
//   400: ErrorResponse
//   409: ErrorResponse

// swagger:parameters AppDeploy
type AppDeployParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.DeployRequest
}

// swagger:route PATCH /namespaces/{Namespace}/applications/{App} Applications AppUpdate
// Update the named `App` in the `Namespace`.
//
// Saves the changes, then answers 200 when nothing has to be redeployed,
// or 202 with the redeploy operation when it does (instances, chart, or a
// restart of a running app).
// responses:
//   200: AppUpdateResponse
//   202: AppOperationResponse
//   409: ErrorResponse

// swagger:parameters AppUpdate
type AppUpdateParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.ApplicationUpdateRequest
}

// swagger:response AppUpdateResponse
type AppUpdateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/validate-cv Applications AppValidateCV
// Validate the chart values of the `App` against its app chart.
// responses:
//   200: AppValidateCVResponse

// swagger:parameters AppValidateCV
type AppValidateCVParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppValidateCVResponse
type AppValidateCVResponse struct {
	// in: body
	Body models.Response
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/export Applications AppExport
// Export the named `App` in the `Namespace`.
// responses:
//   200: AppExportResponse

// swagger:parameters AppExport
type AppExportParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.AppExportRequest
}

// swagger:response AppExportResponse
type AppExportResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /applications/grouped Applications AllAppsGrouped
// Return the applications of every visible namespace, grouped by namespace name.
// Each namespace is paginated independently, replacing one call per namespace.
// Namespaces the user cannot access are omitted entirely.
// responses:
//   200: AllAppsGroupedResponse

// swagger:parameters AllAppsGrouped
type AllAppsGroupedParam struct {
	// Case-insensitive substring match on the application name
	// in: query
	Search string `json:"search"`
	// Page number, 1-based (default 1)
	// in: query
	Page string `json:"page"`
	// Items per page (default 10)
	// in: query
	PageSize string `json:"pageSize"`
}

// swagger:response AllAppsGroupedResponse
type AllAppsGroupedResponse struct {
	// Keyed by namespace name
	// in: body
	Body map[string]AppsPage
}

// AppsPage is one namespace's page of applications.
//
// swagger:model AppsPage
type AppsPage struct {
	Items      []models.App `json:"items"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
	TotalItems int          `json:"totalItems"`
	TotalPages int          `json:"totalPages"`
}

// swagger:route GET /namespaces/{Namespace}/appsmatches Applications AppMatch0
// Return list of names for all applications in the `Namespace`.
// (No prefix == empty prefix == match everything)
// responses:
//   200: AppMatchResponse

// swagger:parameters AppMatch0
type AppMatch0Param struct {
	// in: path
	Namespace string
}

// response: See AppMatch.

// swagger:route GET /namespaces/{Namespace}/applications/{App}/source Applications AppSource
// Return the stored source blob of the named `App` in the `Namespace` as a tar archive.
// Fails with 400 when the application has no stored source.
// responses:
//   200: AppSourceResponse

// swagger:parameters AppSource
type AppSourceParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppSourceResponse
type AppSourceResponse struct {
	// in: body
	Body []byte
}
