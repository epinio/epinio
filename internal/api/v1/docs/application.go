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
	Service models.ApplicationCreateRequest
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
// Waits for the completion of the Tekton PipelineRun resource identified by `StageID` in the `Namespace`.
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
// Create a Tekton PipelineRun resource to stage the named `App` in the `Namespace`.
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
// Create the deployment, service and ingress resources for the named `App` in the `Namespace`.
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
