package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/names"
)

const (
	GiteaURL    = "http://gitea-http.gitea:10080"
	RegistryURL = "registry.epinio-registry/apps"
)

// Stage will create a Tekton PipelineRun resource to stage and start the app
func (hc ApplicationsController) Stage(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	name := params.ByName("app")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	req := models.StageRequest{}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	if name != req.App.Name {
		return singleNewError("name parameter from URL does not match name param in body", http.StatusBadRequest)
	}

	log.Info("staging app", "org", org, "app", req)

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

	// return if another run for this imageID is running; one imageID == one stagingID at the same time
	l, err := client.List(ctx, metav1.ListOptions{LabelSelector: models.EpinioImageIDLabel + "=" + req.Image.ID})
	if err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	// assume that completed pipelineruns are from the past and have a CompletionTime
	for _, pr := range l.Items {
		if pr.Status.CompletionTime == nil {
			return singleNewError("pipelinerun for image ID still running", http.StatusBadRequest)
		}
	}

	app := models.App{
		AppRef: req.App,
		Image:  req.Image,
		Git:    req.Git,
	}

	pr := newPipelineRun(uid, app)
	o, err := client.Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		return singleInternalError(err, fmt.Sprintf("failed to create pipeline run: %#v", o))
	}

	err = createCertificates(ctx, cluster.RestConfig, app)
	if err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	log.Info("staged app", "org", org, "app", app.AppRef, "uid", uid)

	resp := models.StageResponse{Stage: models.NewStage(uid)}
	err = jsonResponse(w, resp)
	if err != nil {
		return singleError(err, http.StatusInternalServerError)
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

func newPipelineRun(uid string, app models.App) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: uid,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Org,
				models.EpinioImageIDLabel:      app.Image.ID,
				models.EpinioStageIDLabel:      uid,
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
							{Name: "revision", Value: app.Git.Revision},
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

func createCertificates(ctx context.Context, cfg *rest.Config, app models.App) error {
	c, err := gitea.New()
	if err != nil {
		return err
	}

	// Create production certificate if it is provided by user
	// else create a local cluster self-signed tls secret.
	if !strings.Contains(c.Domain, "omg.howdoi.website") {
		err = createCertificate(ctx, cfg, app, c.Domain, "letsencrypt-production")
		if err != nil {
			return errors.Wrap(err, "create production ssl certificate failed")
		}
	} else {
		err = createCertificate(ctx, cfg, app, c.Domain, "selfsigned-issuer")
		if err != nil {
			return errors.Wrap(err, "create local ssl certificate failed")
		}
	}
	return nil
}

func createCertificate(ctx context.Context, cfg *rest.Config, app models.App, systemDomain string, issuer string) error {
	data := fmt.Sprintf(`{
		"apiVersion": "cert-manager.io/v1alpha2",
		"kind": "Certificate",
		"metadata": {
			"name": "%s",
			"namespace": "%s"
		},
		"spec": {
			"commonName" : "%s",
			"secretName" : "%s-tls",
			"dnsNames": [
				"%s.%s"
			],
			"issuerRef" : {
				"name" : "%s",
				"kind" : "ClusterIssuer"
			}
		}
        }`, app.Name, app.Org, names.TruncateMD5(fmt.Sprintf("%s.%s", app.Name, systemDomain), 64), app.Name, app.Name, systemDomain, issuer)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return err
	}

	certificateInstanceGVR := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1alpha2",
		Resource: "certificates",
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	_, err = dynamicClient.Resource(certificateInstanceGVR).Namespace(app.Org).
		Create(ctx, obj, metav1.CreateOptions{})
	// Ignore the error if it's about cert already existing.
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
