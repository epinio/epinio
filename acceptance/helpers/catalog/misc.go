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

// Package catalog contains objects and resources, which are used by multiple tests
package catalog

import (
	"encoding/json"
	"os"
	"strings"

	epinioappv1 "github.com/epinio/application/api/v1"
	"github.com/epinio/epinio/acceptance/helpers"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func NginxCatalogService(name string) models.CatalogService {
	values := `{"service": {"type": "ClusterIP"}}`

	return models.CatalogService{
		Meta: models.MetaLite{
			Name: name,
		},
		HelmChart: "nginx",
		HelmRepo: models.HelmRepo{
			Name: "",
			URL:  "https://charts.bitnami.com/bitnami",
		},
		Values: values,
		Settings: map[string]models.ChartSetting{
			"ingress.enabled": {
				Type: "bool",
			},
			"ingress.hostname": {
				Type: "string",
			},
			"sequence": {
				Type: "string",
			},
			"other.sequence": {
				Type: "string",
			},
			"nesting": {
				Type: "map",
			},
		},
	}
}

func CreateCatalogServiceNginx() models.CatalogService {
	catalogService := NginxCatalogService(NewCatalogServiceName())

	CreateCatalogService(catalogService)

	return catalogService
}

func CreateCatalogService(catalogService models.CatalogService) {
	CreateCatalogServiceInNamespace("epinio", catalogService)
}

// Create catalog service in the cluster. The catalog entry is applied via kubectl, after conversion
// into a yaml file
func CreateCatalogServiceInNamespace(namespace string, catalogService models.CatalogService) {
	By("creating catalog entry in " + namespace + ": " + catalogService.Meta.Name)

	sampleServiceFilePath := SampleServiceTmpFile(namespace, catalogService)
	defer os.Remove(sampleServiceFilePath)

	out, err := proc.Kubectl("apply", "-f", sampleServiceFilePath)
	Expect(err).ToNot(HaveOccurred(), out)
}

func DeleteCatalogService(name string) {
	DeleteCatalogServiceFromNamespace("epinio", name)
}

func DeleteCatalogServiceFromNamespace(namespace, name string) {
	By("deleting catalog entry in " + namespace + ": " + name)

	out, err := proc.Kubectl("delete", "-n", namespace, "services.application.epinio.io", name)
	Expect(err).ToNot(HaveOccurred(), out)
}

// Create temp file to hold the catalog service formatted as yaml, and return the path
func SampleServiceTmpFile(namespace string, catalogService models.CatalogService) string {

	// Convert from internal model to CRD structure
	settings := map[string]epinioappv1.ServiceSetting{}
	for key, value := range catalogService.Settings {
		settings[key] = epinioappv1.ServiceSetting{
			Type:    value.Type,
			Minimum: value.Minimum,
			Maximum: value.Maximum,
			Enum:    value.Enum,
		}
	}

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
			Settings:  settings,
		},
	}

	// Check if the installed Epinio version has compatible CRD deployed
	out, err := proc.Kubectl("get", "crd", "services.application.epinio.io", "-o", `jsonpath='{..properties.settings}'`)
	Expect(err).ToNot(HaveOccurred(), out)

	// Delete the Spec.Settings key if the kubectl output is empty - the CRD is not compatible then
	if string(out) == "''" {
		srv.Spec.Settings = nil
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

// Remove a service instance without going through epinio. Code is analogous though
func DeleteService(name, namespace string) {
	sname := names.GenerateResourceName("s", name)

	out, err := proc.Kubectl("delete", "secret", "--namespace", namespace, sname)
	Expect(err).ToNot(HaveOccurred(), out)

	releaseName := names.ServiceReleaseName(name)

	out, err = proc.RunW("helm", "uninstall", releaseName, "--namespace", namespace)
	Expect(err).ToNot(HaveOccurred(), out)
}

// Create a service (instance) from a catalog entry, without going through epinio.  Although the
// code is quite similar.
func CreateService(name, namespace string, catalogService models.CatalogService) {
	CreateServiceX(name, namespace, catalogService, true, false)
}

func CreateUnlabeledService(name, namespace string, catalogService models.CatalogService) {
	CreateServiceX(name, namespace, catalogService, false, false)
}

func CreateServiceWithoutCatalog(name, namespace string, catalogService models.CatalogService) {
	CreateServiceX(name, namespace, catalogService, true, true)
}

// Create a service (instance) from a catalog entry, without going through epinio.  Although the
// code is quite similar.
func CreateServiceX(name, namespace string, catalogService models.CatalogService, label, broken bool) {
	// Phases:
	//   1. secret to represent the service instance
	//   2. helm release representing the active element
	//
	// Effectively a replication of internal/services/instances.go:Create using kubectl and helm cli.

	By("CS setup: " + name)

	// Phase 1, Service Secret.

	By("CS secret")

	var labels map[string]string      // default: nil
	var annotations map[string]string // default: nil

	if label {
		labels = map[string]string{
			"application.epinio.io/catalog-service-name":    catalogService.Meta.Name,
			"application.epinio.io/catalog-service-version": catalogService.AppVersion,
			"application.epinio.io/service-name":            name,
		}
		if broken {
			labels["application.epinio.io/catalog-service-name"] = "missing-catalog-service"
		}
	}

	if len(catalogService.SecretTypes) > 0 {
		types := strings.Join(catalogService.SecretTypes, ",")
		annotations = map[string]string{
			"application.epinio.io/catalog-service-secret-types": types,
		}
	}

	sname := names.GenerateResourceName("s", name)
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Type: "Opaque",
		ObjectMeta: metav1.ObjectMeta{
			Name:        sname,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}

	secretTmpFile := NewTmpName("tmpUserFile") + `.json`
	file, err := os.Create(secretTmpFile)
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(file).Encode(secret)
	Expect(err).ToNot(HaveOccurred())
	defer os.Remove(secretTmpFile)

	out, err := proc.Kubectl("apply", "-f", secretTmpFile)
	Expect(err).ToNot(HaveOccurred(), out)

	// Phase 2, Helm Release.

	By("CS release")

	out, err = proc.RunW("helm", "repo", "add", "bitnami-nginx", catalogService.HelmRepo.URL)
	Expect(err).ToNot(HaveOccurred(), out)

	releaseName := names.ServiceReleaseName(name)

	cmd := []string{
		"upgrade",
		releaseName,
		"bitnami-nginx/" + catalogService.HelmChart,
		"--install", "--namespace", namespace,
	}
	if catalogService.ChartVersion != "" {
		cmd = append(cmd, "--version", catalogService.ChartVersion)
	}
	if catalogService.Values != "" {
		filePath, err := helpers.CreateTmpFile(catalogService.Values)
		Expect(err).ToNot(HaveOccurred())
		cmd = append(cmd, "-f", filePath)
		defer os.Remove(filePath)
	}

	out, err = proc.RunW("helm", cmd...)

	By("CS post release")
	Expect(err).ToNot(HaveOccurred(), out)
}
