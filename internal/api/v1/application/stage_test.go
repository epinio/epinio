package application

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	restclient "k8s.io/client-go/rest"
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

func gitStageRequest(blobUID, url string) models.StageRequest {
	return models.StageRequest{
		App:     models.NewAppRef("app", "ns"),
		BlobUID: blobUID,
		Origin: models.ApplicationOrigin{
			Kind: models.OriginGit,
			Git:  &models.GitRef{URL: url, Revision: "main"},
		},
	}
}

func TestValidateStageRequestOrigin(t *testing.T) {
	cases := []struct {
		name    string
		req     models.StageRequest
		wantErr bool
	}{
		{
			"uploaded blob skips the check",
			gitStageRequest("blob", ""),
			false,
		},
		{"no origin", models.StageRequest{}, false},
		{
			"valid git url",
			gitStageRequest("", "https://github.com/epinio/x"),
			false,
		},
		{"empty git url", gitStageRequest("", ""), true},
		{
			"url without scheme",
			gitStageRequest("", "github.com/epinio/x"),
			true,
		},
		{
			"git origin without repository",
			models.StageRequest{
				Origin: models.ApplicationOrigin{Kind: models.OriginGit},
			},
			true,
		},
	}

	for _, tc := range cases {
		err := validateStageRequestOrigin(tc.req)
		if (err != nil) != tc.wantErr {
			t.Errorf(
				"%s: got error %v, want error %v",
				tc.name,
				err,
				tc.wantErr,
			)
		}
	}
}

func appWithGitOrigin(url string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"origin": map[string]any{
				"git": map[string]any{"repository": url},
			},
		},
	}}
}

func TestSourceOriginPrefersTheRequest(t *testing.T) {
	req := gitStageRequest("", "https://example.com/new.git")
	app := appWithGitOrigin("https://example.com/stored.git")

	origin, err := sourceOrigin(req, app)
	if err != nil {
		t.Fatal(err)
	}
	if origin.Git.URL != "https://example.com/new.git" {
		t.Fatalf("expected the request origin, got %q", origin.Git.URL)
	}
}

func TestSourceOriginFallsBackToTheStoredOrigin(t *testing.T) {
	app := appWithGitOrigin("https://example.com/stored.git")

	origin, err := sourceOrigin(models.StageRequest{}, app)
	if err != nil {
		t.Fatal(err)
	}
	if origin.Git == nil || origin.Git.URL != "https://example.com/stored.git" {
		t.Fatalf("expected the stored origin, got %+v", origin.Git)
	}
}

// originPatches serves the App CR PATCH endpoint and records each body.
func originPatches(t *testing.T) (*kubernetes.Cluster, *[][]byte) {
	t.Helper()
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			bodies = append(bodies, body)
			w.Header().Set("Content-Type", "application/json")
			response := `{"apiVersion":"application.epinio.io/v1",` +
				`"kind":"App","metadata":{"name":"app","namespace":"ns"}}`
			_, _ = w.Write([]byte(response))
		},
	))
	t.Cleanup(srv.Close)

	return &kubernetes.Cluster{
		RestConfig: &restclient.Config{Host: srv.URL},
	}, &bodies
}

func patchedGit(t *testing.T, body []byte) models.GitRef {
	t.Helper()
	var ops []struct {
		Path  string                   `json:"path"`
		Value models.ApplicationOrigin `json:"value"`
	}
	if err := json.Unmarshal(body, &ops); err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Path != "/spec/origin" ||
		ops[0].Value.Git == nil {
		t.Fatalf("unexpected origin patch: %s", body)
	}
	return *ops[0].Value.Git
}

func TestRecordClonedRevisionStampsTheCommit(t *testing.T) {
	cluster, bodies := originPatches(t)
	req := gitStageRequest("", "https://example.com/app.git")
	cloned := &models.GitRef{Branch: "main", Revision: "abc123"}

	if err := recordClonedRevision(
		context.Background(),
		cluster,
		req,
		cloned,
	); err != nil {
		t.Fatal(err)
	}

	if len(*bodies) != 1 {
		t.Fatalf("expected one patch, got %d", len(*bodies))
	}
	git := patchedGit(t, (*bodies)[0])
	if git.Revision != "abc123" || git.Branch != "main" ||
		git.URL != "https://example.com/app.git" {
		t.Fatalf("unexpected stamped origin: %+v", git)
	}
	if req.Origin.Git.Revision != "main" {
		t.Fatal("the request's origin was modified in place")
	}
}

func TestRecordClonedRevisionKeepsTheRequestedBranch(t *testing.T) {
	cluster, bodies := originPatches(t)
	req := gitStageRequest("", "https://example.com/app.git")
	req.Origin.Git.Branch = "release"
	cloned := &models.GitRef{Branch: "other", Revision: "abc123"}

	if err := recordClonedRevision(
		context.Background(),
		cluster,
		req,
		cloned,
	); err != nil {
		t.Fatal(err)
	}

	if git := patchedGit(t, (*bodies)[0]); git.Branch != "release" {
		t.Fatalf("expected branch release, got %q", git.Branch)
	}
}

func TestRecordClonedRevisionSkipsWithoutAClone(t *testing.T) {
	cluster, bodies := originPatches(t)

	cases := []struct {
		name   string
		req    models.StageRequest
		cloned *models.GitRef
	}{
		{"no clone", gitStageRequest("", "https://example.com/a.git"), nil},
		{
			"clone from the stored origin",
			models.StageRequest{},
			&models.GitRef{Revision: "abc123"},
		},
	}

	for _, tc := range cases {
		if err := recordClonedRevision(
			context.Background(),
			cluster,
			tc.req,
			tc.cloned,
		); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}

	if len(*bodies) != 0 {
		t.Fatalf("expected no patches, got %d", len(*bodies))
	}
}
