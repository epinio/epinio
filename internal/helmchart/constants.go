package helmchart

import "github.com/spf13/viper"

const (
	S3ConnectionDetailsSecretName = "epinio-s3-connection-details" // nolint:gosec // Not credentials
	EpinioCertificateName         = "epinio"
	EpinioStageScriptsName        = "epinio-stage-scripts"
	EpinioStageDownload           = "download"
	EpinioStageUnpack             = "unpack"
	EpinioStageBuild              = "build"
)

func Namespace() string {
	return viper.GetString("namespace")
}
