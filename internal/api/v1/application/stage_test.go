package application

import (
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/epinio/epinio/internal/s3manager"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

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

func TestBuildContainerImage(t *testing.T) {
	t.Run("returns BuilderImage when BuildContainerImage is empty", func(t *testing.T) {
		app := stageParam{BuilderImage: "paketo/builder:1", BuildContainerImage: ""}
		got := buildContainerImage(app)
		if got != "paketo/builder:1" {
			t.Fatalf("expected paketo/builder:1, got %q", got)
		}
	})
	t.Run("returns BuildContainerImage when set (Pack path)", func(t *testing.T) {
		app := stageParam{BuilderImage: "paketo/builder:1", BuildContainerImage: "buildpacksio/pack:0.36"}
		got := buildContainerImage(app)
		if got != "buildpacksio/pack:0.36" {
			t.Fatalf("expected buildpacksio/pack:0.36, got %q", got)
		}
	})
}

func TestAssembleStageEnvBUILDER_IMAGE(t *testing.T) {
	base := stageParam{
		AppRef:              models.NewAppRef("app", "ns"),
		BlobUID:             "blob-1",
		RegistryURL:         "reg.io/ns",
		Stage:               models.NewStage("stage-1"),
		PreviousStageID:     "stage-0",
		UserID:              1001,
		GroupID:             1000,
		S3ConnectionDetails: s3manager.ConnectionDetails{UseSSL: true, Endpoint: "s3.example.com", Bucket: "b"},
	}
	previous := base
	previous.Stage = models.NewStage("stage-0")

	t.Run("BUILDER_IMAGE not set when BuildContainerImage is empty", func(t *testing.T) {
		app := base
		app.BuildContainerImage = ""
		app.BuilderImage = "paketo/builder:1"
		env := assembleStageEnv(app, previous)
		for _, e := range env {
			if e.Name == "BUILDER_IMAGE" {
				t.Fatalf("expected no BUILDER_IMAGE env, got %s=%s", e.Name, e.Value)
			}
		}
	})
	t.Run("BUILDER_IMAGE set when BuildContainerImage is set (Pack path)", func(t *testing.T) {
		app := base
		app.BuildContainerImage = "buildpacksio/pack:0.36"
		app.BuilderImage = "paketo/builder-jammy-full:0.3"
		env := assembleStageEnv(app, previous)
		var found string
		for _, e := range env {
			if e.Name == "BUILDER_IMAGE" {
				found = e.Value
				break
			}
		}
		if found != "paketo/builder-jammy-full:0.3" {
			t.Fatalf("expected BUILDER_IMAGE=paketo/builder-jammy-full:0.3, got %q", found)
		}
	})
}

func TestMountDockerSocket(t *testing.T) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{{Name: "source", MountPath: "/workspace"}}

	t.Run("no change when BuildContainerImage is empty", func(t *testing.T) {
		app := stageParam{BuildContainerImage: "", HelmValues: HelmValuesMap{DockerSocketPath: "/var/run/docker.sock"}}
		vol, mounts, buildMounts := mountDockerSocket(app, volumes, volumeMounts)
		if len(vol) != 0 || len(mounts) != 1 || len(buildMounts) != 1 {
			t.Fatalf("expected no extra volume/mounts: vol=%d mounts=%d buildMounts=%d", len(vol), len(mounts), len(buildMounts))
		}
	})
	t.Run("no change when dockerSocketPath is empty", func(t *testing.T) {
		app := stageParam{BuildContainerImage: "pack:latest", HelmValues: HelmValuesMap{DockerSocketPath: ""}}
		vol, mounts, buildMounts := mountDockerSocket(app, volumes, volumeMounts)
		if len(vol) != 0 || len(mounts) != 1 || len(buildMounts) != 1 {
			t.Fatalf("expected no extra volume/mounts: vol=%d mounts=%d buildMounts=%d", len(vol), len(mounts), len(buildMounts))
		}
	})
	t.Run("adds volume and build-only mount when Pack and dockerSocketPath set", func(t *testing.T) {
		app := stageParam{
			BuildContainerImage: "buildpacksio/pack:0.36",
			HelmValues:          HelmValuesMap{DockerSocketPath: "/var/run/docker.sock"},
		}
		vol, mounts, buildMounts := mountDockerSocket(app, volumes, volumeMounts)
		if len(vol) != 1 || vol[0].Name != "docker-socket" {
			t.Fatalf("expected one docker-socket volume, got %d volumes", len(vol))
		}
		if vol[0].HostPath == nil || vol[0].HostPath.Path != "/var/run/docker.sock" {
			t.Fatalf("expected HostPath /var/run/docker.sock, got %+v", vol[0].HostPath)
		}
		if len(mounts) != 1 {
			t.Fatalf("shared mounts should be unchanged: %d", len(mounts))
		}
		if len(buildMounts) != 2 {
			t.Fatalf("expected 2 build mounts (source + docker-socket), got %d", len(buildMounts))
		}
		if buildMounts[1].Name != "docker-socket" || buildMounts[1].MountPath != "/var/run/docker.sock" {
			t.Fatalf("expected docker-socket mount: %+v", buildMounts[1])
		}
	})
}
