package application

import (
	"testing"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResolveSourcePathRequestWins(t *testing.T) {
	app := appWithSourcePath("apps/web")

	path, apiErr := resolveSourcePath(models.StageRequest{SourcePath: "apps/api"}, app)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if path != "apps/api" {
		t.Fatalf("expected apps/api, got %q", path)
	}
}

func TestResolveSourcePathKeepsStoredWhenRequestEmpty(t *testing.T) {
	app := appWithSourcePath("apps/web")

	path, apiErr := resolveSourcePath(models.StageRequest{}, app)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if path != "apps/web" {
		t.Fatalf("expected stored apps/web, got %q", path)
	}
}

func TestResolveSourcePathExplicitRootClearsStored(t *testing.T) {
	app := appWithSourcePath("apps/web")

	path, apiErr := resolveSourcePath(models.StageRequest{SourcePath: models.SourcePathRoot}, app)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if path != "" {
		t.Fatalf("expected empty staging path after reset, got %q", path)
	}
}

func TestResolveSourcePathEmptyWhenNothingStored(t *testing.T) {
	app := &unstructured.Unstructured{Object: map[string]any{}}

	path, apiErr := resolveSourcePath(models.StageRequest{}, app)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if path != "" {
		t.Fatalf("expected empty path, got %q", path)
	}
}

func TestResolveSourcePathRejectsInvalidStored(t *testing.T) {
	app := appWithSourcePath("../escape")

	_, apiErr := resolveSourcePath(models.StageRequest{}, app)
	if apiErr == nil {
		t.Fatal("expected error for invalid stored source path")
	}
}

func TestAssembleStageEnvSourcePathBuildpackOnly(t *testing.T) {
	previous := stageParam{
		AppRef:      models.NewAppRef("prev", "ns"),
		Stage:       models.NewStage("prevstage"),
		RegistryURL: "registry.example",
	}

	buildpack := stageParam{
		AppRef:      models.NewAppRef("app", "ns"),
		Stage:       models.NewStage("stage"),
		RegistryURL: "registry.example",
		BuildMode:   models.BuildModeBuildpack,
		SourcePath:  "apps/web",
	}
	env := assembleStageEnv(buildpack, previous)
	if got := envValue(env, "SOURCE_PATH"); got != "apps/web" {
		t.Fatalf("expected SOURCE_PATH=apps/web for buildpack, got %q", got)
	}

	dockerfile := stageParam{
		AppRef:         models.NewAppRef("app", "ns"),
		Stage:          models.NewStage("stage"),
		RegistryURL:    "registry.example",
		BuildMode:      models.BuildModeDockerfile,
		DockerfilePath: "Dockerfile",
		SourcePath:     "apps/web",
	}
	env = assembleStageEnv(dockerfile, previous)
	if got := envValue(env, "SOURCE_PATH"); got != "" {
		t.Fatalf("expected no SOURCE_PATH in dockerfile mode, got %q", got)
	}
	if got := envValue(env, "DOCKERFILE_PATH"); got != "Dockerfile" {
		t.Fatalf("expected DOCKERFILE_PATH=Dockerfile, got %q", got)
	}

	root := stageParam{
		AppRef:      models.NewAppRef("app", "ns"),
		Stage:       models.NewStage("stage"),
		RegistryURL: "registry.example",
		BuildMode:   models.BuildModeBuildpack,
		SourcePath:  "",
	}
	env = assembleStageEnv(root, previous)
	if got := envValue(env, "SOURCE_PATH"); got != "" {
		t.Fatalf("expected no SOURCE_PATH for empty root, got %q", got)
	}
}

func appWithSourcePath(path string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"sourcepath": path,
		},
	}}
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}
