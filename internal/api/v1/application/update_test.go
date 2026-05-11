package application

import (
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
