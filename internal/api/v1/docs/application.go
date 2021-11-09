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

// swagger:route GET /namespaces/{namespace}/applications application Apps
// Return list of applications in the `namespace`.
// responses:
//   200: AppsResponse

// swagger:parameters Apps
type AppsParam struct {
	Namespace string
}

// swagger:response AppsResponse
type AppsResponse struct {
	// in: body
	Body models.AppList
}

// swagger:route POST /namespaces/{namespace}/applications application AppCreate
// Create the posted new application in the `namespace`.
// responses:
//   200: AppCreateResponse

// swagger:parameters AppCreate
type AppCreateParam struct {
	Namespace string
	// in: body
	Service models.ApplicationCreateRequest
}

// swagger:response AppCreateResponse
type AppCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{namespace}/applications/{app} application AppShow
// Return details of the named `application` in the `namespace`.
// responses:
//   200: AppShowResponse

// swagger:parameters AppShow
type AppShowParam struct {
	Namespace string
	App       string
}

// swagger:response AppShowResponse
type AppShowResponse struct {
	// in: body
	Body models.App
}

// swagger:route GET /namespaces/{namespace}/applications/{app}/logs application AppLogs
// Return logs of the named `application` in the `namespace`.
// responses:
//   200: AppLogsResponse

// swagger:parameters AppLogs
type AppLogsParam struct {
	Namespace string
	App       string
}

// swagger:response AppLogsResponse
type AppLogsResponse struct {
	// in: body
}

// swagger:route GET /namespaces/{namespace}/staging/{stage_id}/logs application StagingLogs
// Return logs of the named `stage_id` in the `namespace`.
// responses:
//   200: StagingLogsResponse

// swagger:parameters StagingLogs
type StagingLogsParam struct {
	Namespace string
	StageId   string
}

// swagger:response StagingLogsResponse
type StagingLogsResponse struct {
	// in: body
}

// swagger:route GET /namespaces/{namespace}/staging/{stage_id}/complete application StagingComplete
// Return logs of the named `stage_id` in the `namespace`.
// responses:
//   200: StagingCompleteResponse

// swagger:parameters StagingComplete
type StagingCompleteParam struct {
	Namespace string
	StageId   string
}

// swagger:response StagingCompleteResponse
type StagingCompleteResponse struct {
	// in: body
	Body models.Response
}

// swagger:route DELETE /namespaces/{namespace}/applications/{application} application AppDelete
// Delete the named `application` in the `namespace`.
// responses:
//   200: AppDeleteResponse

// swagger:parameters AppDelete
type AppDeleteParam struct {
	Namespace string
	App       string
}

// swagger:response AppDeleteResponse
type AppDeleteResponse struct {
	// in: body
	Body models.ApplicationDeleteResponse
}

// swagger:route POST /namespaces/{namespace}/applications/{application}/store application AppUpload
// Store the named `application` in the `namespace`.
// responses:
//   200: AppUploadResponse

// swagger:parameters AppUpload
type AppUploadParam struct {
	Namespace string
	App       string
}

// swagger:response AppUploadResponse
type AppUploadResponse struct {
	// in: body
	Body models.UploadResponse
}

// swagger:route POST /namespaces/{namespace}/applications/{application}/import-git application AppImportGit
// Store the named `application` from a Git repo in the `namespace`.
// responses:
//   200: AppImportGitResponse

// swagger:parameters AppImportGit
type AppImportGitParam struct {
	Namespace string
	App       string
	GitUrl    string
	GitRev    string
}

// swagger:response AppImportGitResponse
type AppImportGitResponse struct {
	// in: body
	Body models.ImportGitResponse
}

// swagger:route POST /namespaces/{namespace}/applications/{application}/stage application AppStage
// Create a Tekton PipelineRun resource to stage the named `application` in the `namespace`.
// responses:
//   200: AppStageResponse

// swagger:parameters AppStage
type AppStageParam struct {
	Namespace string
	App       string
	// in: body
	Body models.StageRequest
}

// swagger:response AppStageResponse
type AppStageResponse struct {
	// in: body
	Body models.StageResponse
}

// swagger:route POST /namespaces/{namespace}/applications/{application}/deploy application AppDeploy
// Create the deployment, service and ingress resources for the named `application` in the `namespace`.
// responses:
//   200: AppDeployResponse

// swagger:parameters AppDeploy
type AppDeployParam struct {
	Namespace string
	App       string
	// in: body
	Body models.DeployRequest
}

// swagger:response AppDeployResponse
type AppDeployResponse struct {
	// in: body
	Body models.DeployResponse
}

// swagger:route PATCH /namespaces/{namespace}/applications/{application} application AppUpdate
// Patch the named `application` in the `namespace`.
// responses:
//   200: AppUpdateResponse

// swagger:parameters AppUpdate
type AppUpdateParam struct {
	Namespace string
	App       string
	// in: body
	Body models.ApplicationUpdateRequest
}

// swagger:response AppUpdateResponse
type AppUpdateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{namespace}/applications/{application}/running application AppRunning
// Wait for the named `application` in the `namespace` to be running.
// responses:
//   200: AppRunningResponse

// swagger:parameters AppRunning
type AppRunningParam struct {
	Namespace string
	App       string
}

// swagger:response AppRunningResponse
type AppRunningResponse struct {
	// in: body
	Body models.Response
}
