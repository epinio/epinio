// Package catalog contains objects and resources, which are used by multiple tests
package catalog

import (
	"encoding/json"
	"os"
	"strings"

	epinioappv1 "github.com/epinio/application/api/v1"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func NginxCatalogService(name string) models.CatalogService {
	By("NGINX: " + name)
	return models.CatalogService{
		Meta: models.MetaLite{
			Name: name,
		},
		HelmChart: "nginx",
		HelmRepo: models.HelmRepo{
			Name: "",
			URL:  "https://charts.bitnami.com/bitnami",
		},
		Values: "{'service': {'type': 'ClusterIP'}}",
	}
}

func CreateCatalogService(catalogService models.CatalogService) {
	CreateCatalogServiceInNamespace("epinio", catalogService)
}

func CreateCatalogServiceInNamespace(namespace string, catalogService models.CatalogService) {
	sampleServiceFilePath := SampleServiceTmpFile(namespace, catalogService)
	defer os.Remove(sampleServiceFilePath)

	By("CCSIN tmp file: " + sampleServiceFilePath)

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
	srv := epinioappv1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: epinioappv1.GroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      catalogService.Meta.Name,
			Namespace: namespace,
		},
		Spec: epinioappv1.ServiceSpec{
			Name:        catalogService.Meta.Name,
			Description: "A simple description of this service.",
			HelmRepo: epinioappv1.HelmRepo{
				URL: catalogService.HelmRepo.URL,
			},
			HelmChart: catalogService.HelmChart,
			Values:    catalogService.Values,
		},
	}

	if len(catalogService.SecretTypes) > 0 {
		srv.ObjectMeta.Annotations = map[string]string{
			services.CatalogServiceSecretTypesAnnotation: strings.Join(catalogService.SecretTypes, ","),
		}
	}

	jsonBytes, err := json.Marshal(srv)
	Expect(err).ToNot(HaveOccurred())

	filePath, err := helpers.CreateTmpFile(string(jsonBytes))
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

func WaitForHelmRelease(namespace, name string) {
	// Wait for the chart release to exist.
	cmd := func() (string, error) {
		return proc.Run("", false, "helm", "get", "all", "-n", namespace, name)
	}
	Eventually(func() error {
		_, err := cmd()
		return err
	}, "5m", "5s").Should(BeNil())

	Eventually(func() string {
		out, _ := cmd()
		return out
	}, "5m", "5s").ShouldNot(MatchRegexp(".*release: not found.*"))
}

func CreateHelmChart(helmChart helmapiv1.HelmChart, wait bool) {
	sampleServiceFilePath := HelmChartTmpFile(helmChart)
	defer os.Remove(sampleServiceFilePath)

	out, err := proc.Kubectl("apply", "-f", sampleServiceFilePath)
	Expect(err).ToNot(HaveOccurred(), out)

	if wait {
		WaitForHelmRelease(
			helmChart.Spec.TargetNamespace,
			helmChart.ObjectMeta.Name)
	}
}

func CreateService(name, namespace string, catalogService models.CatalogService) {
	helmChart := helmapiv1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "helm.cattle.io/v1",
			Kind:       "HelmChart",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.ServiceHelmChartName(name, namespace),
			Namespace: "epinio",
			Labels: map[string]string{
				services.CatalogServiceLabelKey:        catalogService.Meta.Name,
				services.TargetNamespaceLabelKey:       namespace,
				services.CatalogServiceVersionLabelKey: catalogService.AppVersion,
				services.ServiceNameLabelKey:           name,
			},
		},
		Spec: helmapiv1.HelmChartSpec{
			TargetNamespace: namespace,
			Chart:           catalogService.HelmChart,
			Version:         catalogService.ChartVersion,
			Repo:            catalogService.HelmRepo.URL,
			ValuesContent:   catalogService.Values,
		},
	}

	if len(catalogService.SecretTypes) > 0 {
		helmChart.ObjectMeta.Annotations = map[string]string{
			services.CatalogServiceSecretTypesAnnotation: strings.Join(catalogService.SecretTypes, ","),
		}
	}

	CreateHelmChart(helmChart, true)
}
