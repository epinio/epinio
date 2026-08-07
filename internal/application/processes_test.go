// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0.

package application_test

import (
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("application processes", func() {
	It("accepts deployment, cron, and release processes", func() {
		two := int32(2)
		processes := models.ApplicationProcesses{
			"web":       {Kind: models.ApplicationProcessDeployment, Command: []string{"web"}, Replicas: &two, Routes: true},
			"scheduler": {Kind: models.ApplicationProcessCron, Command: []string{"cron"}, Schedule: "*/5 * * * *"},
			"release":   {Kind: models.ApplicationProcessRelease, Command: []string{"migrate"}},
		}

		Expect(application.ValidateProcesses(processes)).To(BeEmpty())
	})

	It("accepts an explicit zero deployment replica count", func() {
		zero := int32(0)
		processes := models.ApplicationProcesses{
			"worker": {Command: []string{"worker"}, Replicas: &zero},
		}

		Expect(application.ValidateProcesses(processes)).To(BeEmpty())
	})

	It("rejects incompatible process fields", func() {
		one := int32(1)
		processes := models.ApplicationProcesses{
			"bad cron": {Kind: models.ApplicationProcessCron, Command: []string{"cron"}, Replicas: &one},
			"release":  {Kind: models.ApplicationProcessRelease},
		}

		Expect(application.ValidateProcesses(processes)).To(HaveLen(4))
	})

	It("rejects multiple routed or release processes", func() {
		processes := models.ApplicationProcesses{
			"web":      {Command: []string{"web"}, Routes: true},
			"admin":    {Command: []string{"admin"}, Routes: true},
			"release":  {Kind: models.ApplicationProcessRelease, Command: []string{"migrate"}},
			"release2": {Kind: models.ApplicationProcessRelease, Command: []string{"migrate-again"}},
		}

		Expect(application.ValidateProcesses(processes)).To(HaveLen(2))
	})

	It("requires a routed process when a process application has external routes", func() {
		processes := models.ApplicationProcesses{
			"worker": {Command: []string{"worker"}},
		}

		Expect(application.ValidateProcessRoutes(processes, []string{"example.test"})).To(MatchError(
			"application routes require one deployment process with routes enabled",
		))
		Expect(application.ValidateProcessRoutes(processes, nil)).To(Succeed())
		Expect(application.ValidateProcessRoutes(nil, []string{"legacy.example.test"})).To(Succeed())
	})

	It("decodes processes from the application CR", func() {
		app := &unstructured.Unstructured{Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"processes": map[string]interface{}{
					"web": map[string]interface{}{
						"kind":     "deployment",
						"command":  []interface{}{"python", "app.py"},
						"replicas": int64(2),
						"routes":   true,
					},
				},
			},
		}}

		processes, err := application.Processes(app)
		Expect(err).ToNot(HaveOccurred())
		Expect(processes["web"].Command).To(Equal([]string{"python", "app.py"}))
		Expect(*processes["web"].Replicas).To(Equal(int32(2)))
	})
})
