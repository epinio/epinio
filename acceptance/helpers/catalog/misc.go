// Package catalog contains objects and resources, which are used by multiple tests
package catalog

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func CreateCatalogService(catalogService models.CatalogService) {
	CreateCatalogServiceInNamespace("epinio", catalogService)
}

func CreateCatalogServiceInNamespace(namespace string, catalogService models.CatalogService) {
	sampleServiceFilePath := SampleServiceTmpFile(namespace, catalogService)
	defer os.Remove(sampleServiceFilePath)

	out, err := proc.Kubectl("apply", "-f", sampleServiceFilePath)
	Expect(err).ToNot(HaveOccurred(), out)
}

func DeleteCatalogService(name string) {
	DeleteCatalogServiceFromNamespace("epinio", name)
}

func DeleteCatalogServiceFromNamespace(namespace, name string) {
	out, err := proc.Kubectl("delete", "-n", namespace, "services.application.epinio.io", name)
	Expect(err).ToNot(HaveOccurred(), out)
}

func SampleServiceTmpFile(namespace string, catalogService models.CatalogService) string {
	serviceYAML := fmt.Sprintf(`
apiVersion: application.epinio.io/v1
kind: Service
metadata:
  name: "%[1]s"
  namespace: "%[2]s"
spec:
  chart: "%[3]s"
  description: |
    A simple description of this service.
  values: "%[5]s"
  helmRepo:
    url: "%[4]s"
  name: %[1]s`,
		catalogService.Name,
		namespace,
		catalogService.HelmChart,
		catalogService.HelmRepo.URL,
		catalogService.Values)

	filePath, err := helpers.CreateTmpFile(serviceYAML)
	Expect(err).ToNot(HaveOccurred())

	return filePath
}

func HelmChartTmpFile(helmChart helmapiv1.HelmChart) string {
	b, err := json.Marshal(helmChart)
	Expect(err).ToNot(HaveOccurred())

	filePath, err := helpers.CreateTmpFile(string(b))
	Expect(err).ToNot(HaveOccurred())

	return filePath
}

func CreateHelmChart(helmChart helmapiv1.HelmChart) {
	sampleServiceFilePath := HelmChartTmpFile(helmChart)
	defer os.Remove(sampleServiceFilePath)

	out, err := proc.Kubectl("apply", "-f", sampleServiceFilePath)
	Expect(err).ToNot(HaveOccurred(), out)

	// Wait for the chart to exist.
	cmd := func() (string, error) {
		return proc.Run("", false, "helm", "get", "all", "-n",
			helmChart.Spec.TargetNamespace,
			helmChart.ObjectMeta.Name)
	}
	Eventually(func() error {
		_, err := cmd()
		return err
	}, "1m", "5s").Should(BeNil())

	Eventually(func() string {
		out, _ := cmd()
		return out
	}, "1m", "5s").ShouldNot(MatchRegexp(".*release: not found.*"))
}

func CreateService(name, namespace string, catalogService models.CatalogService) {
	CreateHelmChart(helmapiv1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "helm.cattle.io/v1",
			Kind:       "HelmChart",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.ServiceHelmChartName(name, namespace),
			Namespace: "epinio",
			Labels: map[string]string{
				services.CatalogServiceLabelKey:  catalogService.Name,
				services.TargetNamespaceLabelKey: namespace,
			},
		},
		Spec: helmapiv1.HelmChartSpec{
			TargetNamespace: namespace,
			Chart:           catalogService.HelmChart,
			Repo:            catalogService.HelmRepo.URL,
		},
	})
}
