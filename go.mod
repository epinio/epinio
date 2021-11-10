module github.com/epinio/epinio

go 1.15

// To avoid CVE-2021-29482
replace github.com/ulikunitz/xz => github.com/ulikunitz/xz v0.5.8

require (
	github.com/Azure/go-autorest/autorest v0.11.17 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.10 // indirect
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/briandowns/spinner v1.12.0
	github.com/codeskyblue/kexec v0.0.0-20180119015717-5a4bed90d99a
	github.com/epinio/application v0.0.0-20211102124051-0d8c19325c97
	github.com/fatih/color v1.12.0
	github.com/gin-contrib/sessions v0.0.3
	github.com/gin-gonic/gin v1.7.4
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/stdr v0.4.0
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/kyokomi/emoji v2.2.4+incompatible
	github.com/mattn/go-colorable v0.1.8
	github.com/mattn/go-isatty v0.0.12
	github.com/maxbrunsfeld/counterfeiter/v6 v6.3.0
	github.com/mholt/archiver/v3 v3.5.0
	github.com/minio/minio-go/v7 v7.0.13
	github.com/novln/docker-parser v1.0.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.7
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/tektoncd/pipeline v0.28.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	gopkg.in/ini.v1 v1.57.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.4
	k8s.io/apiextensions-apiserver v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/metrics v0.21.4
	sigs.k8s.io/yaml v1.2.0
)
