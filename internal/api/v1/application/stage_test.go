package application

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func TestOverrideRegistryHost(t *testing.T) {
	tests := []struct {
		name         string
		registryURL  string
		hostOverride string
		expected     string
	}{
		{
			name:         "replace host keep namespace path",
			registryURL:  "registry.epinio.svc.cluster.local:5000/apps",
			hostOverride: "192.168.49.2:30500",
			expected:     "192.168.49.2:30500/apps",
		},
		{
			name:         "trim trailing slashes",
			registryURL:  "registry.epinio.svc.cluster.local:5000/apps",
			hostOverride: "192.168.49.2:30500/",
			expected:     "192.168.49.2:30500/apps",
		},
		{
			name:         "empty override keeps original",
			registryURL:  "registry.epinio.svc.cluster.local:5000/apps",
			hostOverride: "",
			expected:     "registry.epinio.svc.cluster.local:5000/apps",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := overrideRegistryHost(tc.registryURL, tc.hostOverride)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestJobDoneStateSuccess(t *testing.T) {
	job := batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}

	done, success := jobDoneState([]batchv1.Job{job})
	if !done || !success {
		t.Fatalf("expected done=true, success=true got done=%v success=%v", done, success)
	}
}

func TestJobDoneStateFailure(t *testing.T) {
	job := batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
			},
		},
	}

	done, success := jobDoneState([]batchv1.Job{job})
	if !done || success {
		t.Fatalf("expected done=true, success=false got done=%v success=%v", done, success)
	}
}

func TestJobDoneStatePending(t *testing.T) {
	job := batchv1.Job{}

	done, success := jobDoneState([]batchv1.Job{job})
	if done || success {
		t.Fatalf("expected done=false, success=false got done=%v success=%v", done, success)
	}
}

func TestNewStagingScriptConfigPackFields(t *testing.T) {
	cfg, err := NewStagingScriptConfig(corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "epinio-stage-scripts"},
		Data: map[string]string{
			"builder":     "*",
			"buildEngine": "pack",
			"buildImage":  "buildpacksio/pack:0.38.2",
			"userID":      "1001",
			"groupID":     "1000",
			"env":         "CNB_PLATFORM_API: \"0.11\"",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BuildEngine != "pack" {
		t.Fatalf("expected build engine pack, got %q", cfg.BuildEngine)
	}
	if cfg.BuildImage != "buildpacksio/pack:0.38.2" {
		t.Fatalf("expected build image buildpacksio/pack:0.38.2, got %q", cfg.BuildImage)
	}
}

func TestNewStagingScriptConfigReadsRegistryHostOverride(t *testing.T) {
	cfg, err := NewStagingScriptConfig(corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "epinio-stage-scripts"},
		Data: map[string]string{
			"builder":   "*",
			"userID":    "1001",
			"groupID":   "1000",
			"env":       "CNB_PLATFORM_API: \"0.11\"",
			"buildImage": "paketobuildpacks/builder-jammy-full:latest",
			"staging-values.json": `{
				"serviceAccountName": "epinio-server",
				"dockerSocketPath": "/var/run/docker.sock",
				"registryHostOverride": "192.168.49.2:30500"
			}`,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HelmValues.RegistryHostOverride != "192.168.49.2:30500" {
		t.Fatalf("expected registryHostOverride to be parsed, got %q", cfg.HelmValues.RegistryHostOverride)
	}
}

func TestNewJobRunUsesBuildImageAndBuilderEnv(t *testing.T) {
	app := stageParam{
		AppRef:          models.NewAppRef("my-app", "my-space"),
		BlobUID:         "blob-1",
		BuilderImage:    "paketobuildpacks/builder-jammy-full:latest",
		BuildImage:      "buildpacksio/pack:0.38.2",
		BuildEngine:     "pack",
		DownloadImage:   "amazon/aws-cli:2.13.26",
		UnpackImage:     "ghcr.io/epinio/epinio-unpacker:latest",
		RegistryURL:     "registry.example.com/apps",
		Stage:           models.NewStage("stage-1"),
		PreviousStageID: "prev-stage",
		UserID:          1001,
		GroupID:         1000,
		Scripts:         "epinio-stage-scripts",
	}

	job, _ := newJobRun(app)
	buildContainer := job.Spec.Template.Spec.Containers[0]

	if buildContainer.Image != "buildpacksio/pack:0.38.2" {
		t.Fatalf("expected build container image buildpacksio/pack:0.38.2, got %q", buildContainer.Image)
	}

	foundBuilderImage := false
	for _, ev := range buildContainer.Env {
		if ev.Name == "BUILDERIMAGE" && ev.Value == "paketobuildpacks/builder-jammy-full:latest" {
			foundBuilderImage = true
			break
		}
	}
	if !foundBuilderImage {
		t.Fatalf("expected BUILDERIMAGE to be passed to build container env")
	}
}
