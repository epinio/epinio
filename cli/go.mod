module github.com/suse/carrier/cli

go 1.13

replace (
	github.com/coreos/bbolt => github.com/coreos/bbolt v1.3.0
	google.golang.org/grpc => google.golang.org/grpc v1.29.1
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.6.4
)

require (
	code.cloudfoundry.org/eirini v0.0.0-20201118000750-3dcf72f6ed2f
	code.gitea.io/sdk/gitea v0.13.2
	github.com/briandowns/spinner v1.12.0
	github.com/codeskyblue/kexec v0.0.0-20180119015717-5a4bed90d99a
	github.com/fatih/color v1.9.0
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/wire v0.4.0
	github.com/kyokomi/emoji v2.2.4+incompatible
	github.com/maxbrunsfeld/counterfeiter/v6 v6.3.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/otiai10/copy v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.7
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	golang.org/x/mod v0.4.0 // indirect
	golang.org/x/sys v0.0.0-20201214210602-f9fddec55a1e // indirect
	golang.org/x/tools v0.0.0-20201215192005-fa10ef0b8743 // indirect
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)
