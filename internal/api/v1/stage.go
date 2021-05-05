package v1

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
)

const (
	GiteaURL    = "http://gitea-http.gitea:10080"
	RegistryURL = "registry.epinio-registry/apps"
)

// Stage will create a Tekton PipelineRun resource to stage and start the app
func (hc ApplicationsController) Stage(w http.ResponseWriter, r *http.Request) APIErrors {
	log := tracelog.Logger(r.Context())

	params := httprouter.ParamsFromContext(r.Context())
	org := params.ByName("org")
	name := params.ByName("app")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	app := gitea.App{}
	if err := json.Unmarshal(bodyBytes, &app); err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	if name != app.Name {
		return singleNewError("name parameter from URL does not match name param in body", http.StatusBadRequest)
	}

	log.Info("staging app", "org", org, "app", app)

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return singleInternalError(err, "failed to get access to a kube client")
	}

	cs, err := versioned.NewForConfig(cluster.RestConfig)
	if err != nil {
		return singleInternalError(err, "failed to get access to a tekton client")
	}
	client := cs.TektonV1beta1().PipelineRuns(deployments.TektonStagingNamespace)

	uid, err := uid()
	if err != nil {
		return singleInternalError(err, "failed to get access to a tekton client")
	}

	pr := newPipelineRun(uid, app)
	o, err := client.Create(r.Context(), pr, v1.CreateOptions{})
	if err != nil {
		return singleInternalError(err, fmt.Sprintf("failed to create pipeline run: %#v", o))
	}

	err = NewAppResponse("ok", app).Write(w)
	if err != nil {
		return NewAPIErrors(InternalError(err))
	}

	return nil
}

func uid() (string, error) {
	randBytes := make([]byte, 16)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", err
	}

	a := fnv.New64()
	_, err = a.Write([]byte(string(randBytes)))
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(a.Sum(nil)), nil
}

func newPipelineRun(uid string, app gitea.App) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: v1.ObjectMeta{
			Name: app.Name + uid,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Org,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "staging",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "staging-triggers-admin",
			PipelineRef:        &v1beta1.PipelineRef{Name: "staging-pipeline"},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name: "source",
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
								corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("1Gi"),
							}},
						},
					},
				},
			},
			Resources: []v1beta1.PipelineResourceBinding{
				{
					Name: "source-repo",
					ResourceSpec: &v1alpha1.PipelineResourceSpec{
						Type: v1alpha1.PipelineResourceTypeGit,
						Params: []v1alpha1.ResourceParam{
							{Name: "revision", Value: app.Repo.Revision},
							{Name: "url", Value: app.GitURL(GiteaURL)},
						},
					},
				},
				{
					Name: "image",
					ResourceSpec: &v1alpha1.PipelineResourceSpec{
						Type: v1alpha1.PipelineResourceTypeImage,
						Params: []v1alpha1.ResourceParam{
							{Name: "url", Value: app.ImageURL(RegistryURL)},
						},
					},
				},
			},
		},
	}
}
