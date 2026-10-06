package helm

import (
	"context"
	"os"
	"path/filepath"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helm/helmtest"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// Keep the specs from reading and writing the registry logins of the user running them.
var _ = BeforeSuite(func() {
	restore, err := helmtest.IsolateHelmConfig()
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(restore)
})

var _ = Describe("Charts in a registry", func() {
	const (
		epinioNamespace = "epinio"
		username        = "admin"
		password        = "changeme"
	)

	var (
		ctx        context.Context
		reg        *helmtest.Registry
		fakeClient *fake.Clientset
		cluster    *kubernetes.Cluster
		tmpDir     string
	)

	// setRegistrySecret makes the registry credentials of the cluster hold the given registry URLs,
	// all with the credentials of the fake registry.
	setRegistrySecret := func(urls ...string) {
		auths := ""
		for i, u := range urls {
			if i > 0 {
				auths += ","
			}
			auths += `"` + u + `":{"username":"` + username + `","password":"` + password + `"}`
		}
		_, err := fakeClient.CoreV1().Secrets(epinioNamespace).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: registry.CredentialsSecretName, Namespace: epinioNamespace},
			Data:       map[string][]byte{".dockerconfigjson": []byte(`{"auths":{` + auths + `}}`)},
			Type:       corev1.SecretTypeDockerConfigJson,
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}

	// saveChart writes a chart archive into a directory of its own, and returns its path.
	saveChart := func(name, version string) string {
		dir, err := os.MkdirTemp(tmpDir, "chart-")
		Expect(err).ToNot(HaveOccurred())

		path, err := chartutil.Save(&chart.Chart{
			Metadata: &chart.Metadata{APIVersion: "v2", Name: name, Version: version},
		}, dir)
		Expect(err).ToNot(HaveOccurred())
		return path
	}

	helmClient := func() *SynchronizedClient {
		client, err := GetHelmClient(cluster.RestConfig, "")
		Expect(err).ToNot(HaveOccurred())
		return client
	}

	// seed stores a chart for the AppChart in the registry, the way the server does.
	seed := func(appChartName, chartName, version string) {
		r, err := OpenChartRegistry(ctx, cluster, appChartName)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		ExpectWithOffset(1, r.Push(saveChart(chartName, version))).To(Succeed())
	}

	appChartFor := func(appChartName, chartRef string) *models.AppChartFull {
		return &models.AppChartFull{AppChart: models.AppChart{
			HelmRepo:  "oci://" + reg.Host() + "/" + ChartRepositoryPath + "/" + appChartName,
			HelmChart: chartRef,
		}}
	}

	BeforeEach(func() {
		ctx = context.Background()
		reg = helmtest.NewRegistry(username, password)
		DeferCleanup(reg.Close)

		fakeClient = fake.NewSimpleClientset()
		cluster = &kubernetes.Cluster{
			Kubectl:    fakeClient,
			RestConfig: &rest.Config{Host: "http://127.0.0.1:1"},
		}
		tmpDir = GinkgoT().TempDir()

		oldNamespace := viper.GetString("namespace")
		viper.Set("namespace", epinioNamespace)
		DeferCleanup(func() { viper.Set("namespace", oldNamespace) })

		oldLogger := helpers.Logger
		helpers.Logger = zap.NewNop().Sugar()
		DeferCleanup(func() { helpers.Logger = oldLogger })
	})

	Describe("SynchronizedClient.Pull", func() {
		var (
			client *SynchronizedClient
			repo   string
		)

		push := func(name, version string) {
			_, err := client.Push(saveChart(name, version), repo, action.WithInsecureSkipTLSVerify(true))
			ExpectWithOffset(1, err).ToNot(HaveOccurred())
		}

		BeforeEach(func() {
			client = helmClient()
			repo = "oci://" + reg.Host() + "/charts"

			Expect(client.RegistryLogin(reg.Host(), username, password, action.WithInsecure(true))).To(Succeed())
		})

		loadedVersion := func(archive string) string {
			ch, err := loader.Load(archive)
			ExpectWithOffset(1, err).ToNot(HaveOccurred())
			return ch.Metadata.Name + ":" + ch.Metadata.Version
		}

		It("downloads the chart with the given version", func() {
			push("mychart", "0.1.0")
			push("mychart", "0.2.0")

			dir := GinkgoT().TempDir()
			archive, err := client.Pull(repo+"/mychart", "0.1.0", dir)
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Dir(archive)).To(Equal(dir))
			Expect(loadedVersion(archive)).To(Equal("mychart:0.1.0"))
		})

		It("downloads the latest version when none is given", func() {
			push("mychart", "0.1.0")
			push("mychart", "0.2.0")

			archive, err := client.Pull(repo+"/mychart", "", GinkgoT().TempDir())
			Expect(err).ToNot(HaveOccurred())
			Expect(loadedVersion(archive)).To(Equal("mychart:0.2.0"))
		})

		It("fails for a version which does not exist", func() {
			push("mychart", "0.1.0")

			_, err := client.Pull(repo+"/mychart", "9.9.9", GinkgoT().TempDir())
			Expect(err).To(HaveOccurred())
		})

		It("fails for a chart which does not exist", func() {
			_, err := client.Pull(repo+"/nothere", "0.1.0", GinkgoT().TempDir())
			Expect(err).To(HaveOccurred())
		})

		It("fails when the destination already holds another chart archive", func() {
			push("mychart", "0.1.0")

			dir := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(dir, "other-1.0.0.tgz"), []byte("x"), 0600)).To(Succeed())

			_, err := client.Pull(repo+"/mychart", "0.1.0", dir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected exactly one chart archive"))
		})

		It("fails without a login to a registry demanding one", func() {
			other := helmtest.NewRegistry(username, password)
			DeferCleanup(other.Close)

			_, err := client.Pull("oci://"+other.Host()+"/charts/mychart", "0.1.0", GinkgoT().TempDir())
			Expect(err).To(HaveOccurred())
		})

		It("fails for a client of the wrong type", func() {
			_, err := (&SynchronizedClient{}).Pull(repo+"/mychart", "0.1.0", GinkgoT().TempDir())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("helm client is not of the right type"))
		})
	})

	Describe("ChartRegistry", func() {
		BeforeEach(func() {
			setRegistrySecret(reg.Host())
		})

		It("keeps the chart of each application chart in a repository of its own", func() {
			first, err := OpenChartRegistry(ctx, cluster, "first")
			Expect(err).ToNot(HaveOccurred())
			Expect(first.Repository()).To(Equal("oci://" + reg.Host() + "/epinio-charts/first"))

			second, err := OpenChartRegistry(ctx, cluster, "second")
			Expect(err).ToNot(HaveOccurred())
			Expect(second.Repository()).To(Equal("oci://" + reg.Host() + "/epinio-charts/second"))

			Expect(first.Push(saveChart("mychart", "0.1.0"))).To(Succeed())

			Expect(reg.HasChart("epinio-charts/first/mychart", "0.1.0")).To(BeTrue())
			Expect(reg.HasChart("epinio-charts/second/mychart", "0.1.0")).To(BeFalse())

			// The same chart name and version, for a different application chart, does not
			// collide. It is not seen as stored, and goes to its own repository.
			stored, err := second.HasChart("mychart", "0.1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(stored).To(BeFalse())

			Expect(second.Push(saveChart("mychart", "0.1.0"))).To(Succeed())
			Expect(reg.HasChart("epinio-charts/second/mychart", "0.1.0")).To(BeTrue())
		})

		It("knows which chart versions it holds", func() {
			r, err := OpenChartRegistry(ctx, cluster, "myapp")
			Expect(err).ToNot(HaveOccurred())

			stored, err := r.HasChart("mychart", "0.1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(stored).To(BeFalse())

			Expect(r.Push(saveChart("mychart", "0.1.0"))).To(Succeed())

			stored, err = r.HasChart("mychart", "0.1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(stored).To(BeTrue())

			stored, err = r.HasChart("mychart", "0.2.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(stored).To(BeFalse())

			stored, err = r.HasChart("otherchart", "0.1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(stored).To(BeFalse())
		})

		It("finds versions with build metadata, stored under a tag with `_` for `+`", func() {
			r, err := OpenChartRegistry(ctx, cluster, "myapp")
			Expect(err).ToNot(HaveOccurred())

			Expect(r.Push(saveChart("mychart", "1.0.0+build.1"))).To(Succeed())
			Expect(reg.Tags("epinio-charts/myapp/mychart")).To(Equal([]string{"1.0.0_build.1"}))

			stored, err := r.HasChart("mychart", "1.0.0+build.1")
			Expect(err).ToNot(HaveOccurred())
			Expect(stored).To(BeTrue())
		})

		It("fails when the chart cannot be pushed", func() {
			r, err := OpenChartRegistry(ctx, cluster, "myapp")
			Expect(err).ToNot(HaveOccurred())

			err = r.Push(filepath.Join(tmpDir, "does-not-exist.tgz"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pushing the chart"))
		})

		It("fails when the registry rejects the credentials", func() {
			reg.SetPassword("something-else")

			_, err := OpenChartRegistry(ctx, cluster, "myapp")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("logging into the registry"))
		})

		It("fails when there are no registry credentials", func() {
			Expect(fakeClient.CoreV1().Secrets(epinioNamespace).
				Delete(ctx, registry.CredentialsSecretName, metav1.DeleteOptions{})).To(Succeed())

			_, err := OpenChartRegistry(ctx, cluster, "myapp")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("getting the registry connection details"))
		})

		It("fails when the registry credentials hold no public url", func() {
			Expect(fakeClient.CoreV1().Secrets(epinioNamespace).
				Delete(ctx, registry.CredentialsSecretName, metav1.DeleteOptions{})).To(Succeed())
			setRegistrySecret("127.0.0.1:30500")

			_, err := OpenChartRegistry(ctx, cluster, "myapp")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no registry url found"))
		})
	})

	Describe("FetchOCIChartArchive", func() {
		BeforeEach(func() {
			setRegistrySecret(reg.Host())
			seed("myapp", "mychart", "0.1.0")
		})

		It("downloads the chart, and logs into the registry for it", func() {
			archive, cleanup, err := FetchOCIChartArchive(ctx, cluster, appChartFor("myapp", "mychart:0.1.0"))
			Expect(err).ToNot(HaveOccurred())

			ch, err := loader.Load(archive)
			Expect(err).ToNot(HaveOccurred())
			Expect(ch.Metadata.Name).To(Equal("mychart"))
			Expect(ch.Metadata.Version).To(Equal("0.1.0"))

			dir := filepath.Dir(archive)
			Expect(dir).To(BeADirectory())
			cleanup()
			Expect(dir).ToNot(BeADirectory())
		})

		It("downloads the latest version when the chart has none", func() {
			seed("myapp", "mychart", "0.2.0")

			archive, cleanup, err := FetchOCIChartArchive(ctx, cluster, appChartFor("myapp", "mychart"))
			Expect(err).ToNot(HaveOccurred())
			defer cleanup()

			ch, err := loader.Load(archive)
			Expect(err).ToNot(HaveOccurred())
			Expect(ch.Metadata.Version).To(Equal("0.2.0"))
		})

		It("leaves nothing behind when the chart cannot be pulled", func() {
			scratch := GinkgoT().TempDir()
			// os.MkdirTemp honors these, the directory is where the download would go.
			GinkgoT().Setenv("TMPDIR", scratch)
			GinkgoT().Setenv("TMP", scratch)
			GinkgoT().Setenv("TEMP", scratch)

			archive, cleanup, err := FetchOCIChartArchive(ctx, cluster, appChartFor("myapp", "nothere:0.1.0"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pulling the OCI chart"))
			Expect(archive).To(BeEmpty())
			Expect(cleanup).ToNot(BeNil())
			cleanup()

			entries, err := os.ReadDir(scratch)
			Expect(err).ToNot(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("fails when the registry rejects the credentials", func() {
			reg.SetPassword("something-else")

			_, cleanup, err := FetchOCIChartArchive(ctx, cluster, appChartFor("myapp", "mychart:0.1.0"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("logging into the OCI chart registry"))
			cleanup()
		})
	})

	Describe("loadChartIdentity", func() {
		var client *SynchronizedClient

		BeforeEach(func() {
			setRegistrySecret(reg.Host())
			seed("myapp", "mychart", "0.1.0")
			seed("myapp", "mychart", "0.2.0")

			client = helmClient()
		})

		It("resolves the name and version of an OCI chart, without a login made before", func() {
			appChart := appChartFor("myapp", "mychart:0.1.0")

			name, version, err := loadChartIdentity(ctx, cluster, client, appChart,
				ociChartRef(appChart.HelmRepo, "mychart"), "0.1.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(name).To(Equal("mychart"))
			Expect(version).To(Equal("0.1.0"))
		})

		It("resolves the latest version for an OCI chart without version", func() {
			appChart := appChartFor("myapp", "mychart")

			name, version, err := loadChartIdentity(ctx, cluster, client, appChart,
				ociChartRef(appChart.HelmRepo, "mychart"), "")
			Expect(err).ToNot(HaveOccurred())
			Expect(name).To(Equal("mychart"))
			Expect(version).To(Equal("0.2.0"))
		})

		It("fails for a chart which is not in the registry", func() {
			appChart := appChartFor("myapp", "nothere:0.1.0")

			_, _, err := loadChartIdentity(ctx, cluster, client, appChart,
				ociChartRef(appChart.HelmRepo, "nothere"), "0.1.0")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pulling the OCI chart"))
		})

		It("fails when the registry rejects the credentials", func() {
			reg.SetPassword("something-else")
			appChart := appChartFor("myapp", "mychart:0.1.0")

			_, _, err := loadChartIdentity(ctx, cluster, client, appChart,
				ociChartRef(appChart.HelmRepo, "mychart"), "0.1.0")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("logging into the OCI chart registry"))
		})

		It("loads a chart from a local archive when the application chart has no repository", func() {
			archive := saveChart("localchart", "3.2.1")
			appChart := &models.AppChartFull{AppChart: models.AppChart{HelmChart: archive}}

			name, version, err := loadChartIdentity(ctx, cluster, client, appChart, archive, "")
			Expect(err).ToNot(HaveOccurred())
			Expect(name).To(Equal("localchart"))
			Expect(version).To(Equal("3.2.1"))
		})
	})
})
