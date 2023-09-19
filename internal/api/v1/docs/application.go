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

// swagger:route GET /applications application AllApps
// Return list of applications in all namespaces.
// responses:
//   200: AppsResponse

// swagger:parameters AllApps
type AllAppsParam struct{}

// response: See Apps.

// swagger:route GET /namespaces/{Namespace}/applications application Apps
// Return list of applications in the `Namespace`.
// responses:
//   200: AppsResponse

// swagger:parameters Apps
type AppsParam struct {
	// in: path
	Namespace string
}

// swagger:response AppsResponse
type AppsResponse struct {
	// in: body
	Body models.AppList
}

// swagger:route POST /namespaces/{Namespace}/applications application AppCreate
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

// swagger:route GET /namespaces/{Namespace}/applications/{App} application AppShow
// Return details of the named `App` in the `Namespace`.
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

// swagger:route GET /namespace/{Namespace}/appsmatches/{Pattern} application AppMatch
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

// swagger:route GET /namespaces/{Namespace}/applications/{App}/part/{Part} application AppPart
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

// swagger:route GET /namespaces/{Namespace}/applications/{App}/logs application AppLogs
// Return logs of the named `App` in the `Namespace` streamed over a websocket.
// responses:
//   200: AppLogsResponse

// swagger:parameters AppLogs
type AppLogsParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppLogsResponse
type AppLogsResponse struct{}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/exec application AppExec
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

// swagger:route GET /namespaces/{Namespace}/applications/{App}/portforward application AppPortForward
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

// swagger:route GET /namespaces/{Namespace}/staging/{StageID}/logs application StagingLogs
// Return logs of the named `StageID` in the `Namespace` streamed over a websocket.
// responses:
//   200: StagingLogsResponse

// swagger:parameters StagingLogs
type StagingLogsParam struct {
	// in: path
	Namespace string
	// in: path
	StageID string
}

// swagger:response StagingLogsResponse
type StagingLogsResponse struct{}

// swagger:route GET /namespaces/{Namespace}/staging/{StageID}/complete application StagingComplete
// Waits for the completion of the staging process identified by `StageID` in the `Namespace`.
// responses:
//   200: StagingCompleteResponse

// swagger:parameters StagingComplete
type StagingCompleteParam struct {
	// in: path
	Namespace string
	// in: path
	StageID string
}

// swagger:response StagingCompleteResponse
type StagingCompleteResponse struct {
	// in: body
	Body models.Response
}

// swagger:route DELETE /namespaces/{Namespace}/applications AppBatchDelete
// Delete the named `Applications` in the `Namespace`.
// responses:
//   200: AppDeleteResponse

// swagger:route DELETE /namespaces/{Namespace}/applications/{App} application AppDelete
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

// swagger:route POST /namespaces/{Namespace}/applications/{App}/store application AppUpload
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

// swagger:route POST /namespaces/{Namespace}/applications/{App}/restart application AppRestart
// Restart the named `App` in the `Namespace`.
// responses:
//   200: AppRestartResponse

// swagger:parameters AppRestart
type AppRestartParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppRestartResponse
type AppRestartResponse struct {
	// in: body
	Body models.Response
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/import-git application AppImportGit
// Store the named `App` from a Git repo in the `Namespace`.
// responses:
//   200: AppImportGitResponse

// swagger:parameters AppImportGit
type AppImportGitParam struct {
	// in: path
	Namespace string
	// in: path
	App    string
	GitUrl string
	GitRev string
}

// swagger:response AppImportGitResponse
type AppImportGitResponse struct {
	// in: body
	Body models.ImportGitResponse
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/stage application AppStage
// Create the resources needed to stage the named `App` in the `Namespace`.
// responses:
//   200: AppStageResponse

// swagger:parameters AppStage
type AppStageParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.StageRequest
}

// swagger:response AppStageResponse
type AppStageResponse struct {
	// in: body
	Body models.StageResponse
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/deploy application AppDeploy
// Create the deployment, configuration and ingress resources for the named `App` in the `Namespace`.
// responses:
//   200: AppDeployResponse

// swagger:parameters AppDeploy
type AppDeployParam struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.DeployRequest
}

// swagger:response AppDeployResponse
type AppDeployResponse struct {
	// in: body
	Body models.DeployResponse
}

// swagger:route PATCH /namespaces/{Namespace}/applications/{App} application AppUpdate
// Patch the named `App` in the `Namespace`.
// responses:
//   200: AppUpdateResponse

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

// swagger:route GET /namespaces/{Namespace}/applications/{App}/running application AppRunning
// Wait for the named `App` in the `Namespace` to be running.
// responses:
//   200: AppRunningResponse

// swagger:parameters AppRunning
type AppRunningParam struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response AppRunningResponse
type AppRunningResponse struct {
	// in: body
	Body models.Response
}

// swagger:route POST /namespaces/{Namespace}/applications/{App}/validate-cv application AppValidateCV
// Validate the chart values configured for the named `App` in the given `Namespace` against the
// configured app chart.
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

// swagger:route POST /namespaces/{Namespace}/applications/{App}/export application AppExport
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
