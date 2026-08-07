// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0.

package application

import (
	"fmt"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ValidateProcesses validates the deliberately small multi-process POC model.
// Kubernetes remains responsible for validating the cron schedule itself.
func ValidateProcesses(processes models.ApplicationProcesses) []error {
	var issues []error
	routeProcesses := 0
	releaseProcesses := 0

	for name, process := range processes {
		if problems := validation.IsDNS1123Label(name); len(problems) > 0 {
			issues = append(issues, fmt.Errorf("process %q has an invalid name: %s", name, problems[0]))
		}
		if len(process.Command) == 0 {
			issues = append(issues, fmt.Errorf("process %q must define a command", name))
		}

		kind := process.Kind
		if kind == "" {
			kind = models.ApplicationProcessDeployment
		}

		switch kind {
		case models.ApplicationProcessDeployment:
			if process.Replicas != nil && *process.Replicas < 0 {
				issues = append(issues, fmt.Errorf("process %q replicas must be zero or greater", name))
			}
			if process.Schedule != "" {
				issues = append(issues, fmt.Errorf("deployment process %q cannot define a schedule", name))
			}
		case models.ApplicationProcessCron:
			if process.Schedule == "" {
				issues = append(issues, fmt.Errorf("cron process %q must define a schedule", name))
			}
			if process.Replicas != nil {
				issues = append(issues, fmt.Errorf("cron process %q cannot define replicas", name))
			}
			if process.Routes {
				issues = append(issues, fmt.Errorf("cron process %q cannot receive routes", name))
			}
		case models.ApplicationProcessRelease:
			releaseProcesses++
			if process.Replicas != nil {
				issues = append(issues, fmt.Errorf("release process %q cannot define replicas", name))
			}
			if process.Schedule != "" {
				issues = append(issues, fmt.Errorf("release process %q cannot define a schedule", name))
			}
			if process.Routes {
				issues = append(issues, fmt.Errorf("release process %q cannot receive routes", name))
			}
		default:
			issues = append(issues, fmt.Errorf("process %q has unknown kind %q", name, process.Kind))
		}

		if process.Routes {
			routeProcesses++
		}
	}

	if routeProcesses > 1 {
		issues = append(issues, fmt.Errorf("only one deployment process may receive application routes"))
	}
	if releaseProcesses > 1 {
		issues = append(issues, fmt.Errorf("only one release process is supported"))
	}

	return issues
}

// ValidateProcessRoutes ensures that external application routes have a
// process Service to target. A nil process map is the legacy single-process
// model and remains valid for backward compatibility.
func ValidateProcessRoutes(processes models.ApplicationProcesses, routes []string) error {
	if processes == nil || len(routes) == 0 {
		return nil
	}

	for _, process := range processes {
		if process.Routes {
			return nil
		}
	}

	return fmt.Errorf("application routes require one deployment process with routes enabled")
}
