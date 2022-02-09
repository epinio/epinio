package helmchart

const (
	S3ConnectionDetailsSecretName = "epinio-s3-connection-details" // nolint:gosec // Not credentials
	StagingNamespace              = "epinio-staging"
	EpinioNamespace               = "epinio"
	EpinioCertificateName         = "epinio"
	EpinioStageScriptsName        = "epinio-stage-scripts"
	EpinioStageDownload           = "download"
	EpinioStageUnpack             = "unpack"
	EpinioStageBuild              = "build"
)
