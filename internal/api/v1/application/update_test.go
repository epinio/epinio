package application

import (
	"context"
	"testing"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func TestEnvReplaceFlag(t *testing.T) {
	t.Run("nil defaults to merge", func(t *testing.T) {
		if envReplaceFlag(nil) {
			t.Fatalf("expected replace flag to default to false")
		}
	})

	t.Run("false pointer merges", func(t *testing.T) {
		val := false
		if envReplaceFlag(&val) {
			t.Fatalf("expected replace flag false to merge")
		}
	})

	t.Run("true pointer replaces", func(t *testing.T) {
		val := true
		if !envReplaceFlag(&val) {
			t.Fatalf("expected replace flag true to replace")
		}
	})
}

func TestAppInstancesChanged(t *testing.T) {
	t.Run("no requested instances", func(t *testing.T) {
		current := int32(1)
		app := &models.App{
			Configuration: models.ApplicationConfiguration{Instances: &current},
		}

		if appInstancesChanged(app, nil) {
			t.Fatalf("expected no change when request does not include instances")
		}
	})

	t.Run("current instances unknown", func(t *testing.T) {
		requested := int32(1)
		app := &models.App{}

		if !appInstancesChanged(app, &requested) {
			t.Fatalf("expected change when current instances are unknown")
		}
	})

	t.Run("requested instances match current", func(t *testing.T) {
		current := int32(2)
		requested := int32(2)
		app := &models.App{
			Configuration: models.ApplicationConfiguration{Instances: &current},
		}

		if appInstancesChanged(app, &requested) {
			t.Fatalf("expected no change when requested instances match current")
		}
	})

	t.Run("requested instances differ from current", func(t *testing.T) {
		current := int32(0)
		requested := int32(1)
		app := &models.App{
			Configuration: models.ApplicationConfiguration{Instances: &current},
		}

		if !appInstancesChanged(app, &requested) {
			t.Fatalf("expected change when requested instances differ from current")
		}
	})
}

func TestChooseRedeploy(t *testing.T) {
	one, two := int32(1), int32(2)
	no := false
	running := &models.App{
		Workload:      &models.AppDeployment{},
		Status:        models.ApplicationRunning,
		Configuration: models.ApplicationConfiguration{Instances: &one},
	}
	stopped := &models.App{
		Workload:      &models.AppDeployment{},
		Status:        models.ApplicationError,
		Configuration: models.ApplicationConfiguration{Instances: &one},
	}
	undeployed := &models.App{}

	cases := []struct {
		name     string
		app      *models.App
		req      models.ApplicationUpdateRequest
		chart    bool
		redeploy bool
	}{
		{
			name:     "no-restart skips even a chart change",
			app:      running,
			req:      models.ApplicationUpdateRequest{Restart: &no},
			chart:    true,
			redeploy: false,
		},
		{
			name:     "instances changed deploys",
			app:      running,
			req:      models.ApplicationUpdateRequest{Instances: &two},
			redeploy: true,
		},
		{
			name:     "chart change on a deployed app deploys",
			app:      stopped,
			chart:    true,
			redeploy: true,
		},
		{
			name:     "running app restarts",
			app:      running,
			redeploy: true,
		},
		{
			name:     "deployed but not running is skipped",
			app:      stopped,
			redeploy: false,
		},
		{
			name:     "undeployed with instances deploys",
			app:      undeployed,
			req:      models.ApplicationUpdateRequest{Instances: &one},
			redeploy: true,
		},
		{
			name:     "undeployed with no instances is done",
			app:      undeployed,
			redeploy: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := chooseRedeploy(
				context.Background(),
				nil,
				tc.app,
				tc.req,
				tc.chart,
				"user",
			)
			if (got != nil) != tc.redeploy {
				t.Fatalf("redeploy = %v, want %v", got != nil, tc.redeploy)
			}
		})
	}
}
