package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
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
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/names"
)

const (
	RegistryURL      = "registry.192.168.99.240.omg.howdoi.website/apps"
	DefaultInstances = int32(1)
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
		return NewAPIErrors(NewAPIError("Failed to construct an Application from the request", err.Error(), http.StatusBadRequest))
	}

	if name != req.App.Name {
		return singleNewError("name parameter from URL does not match name param in body", http.StatusBadRequest)
	}

	if req.Instances != nil && *req.Instances < 0 {
		return APIErrors{NewAPIError(
			"instances param should be integer equal or greater than zero",
			"", http.StatusBadRequest)}
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

	uid, err := randstr.Hex16()
	if err != nil {
		return singleInternalError(err, "failed to generate a uid")
	}

	l, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", req.App.Name, req.App.Org),
	})
	if err != nil {
		return singleError(err, http.StatusInternalServerError)
	}

	// assume that completed pipelineruns are from the past and have a CompletionTime
	for _, pr := range l.Items {
		if pr.Status.CompletionTime == nil {
			return singleNewError("pipelinerun for image ID still running", http.StatusBadRequest)
		}
	}

	// find out the instances
	var instances int32
	if req.Instances != nil {
		instances = int32(*req.Instances)
	} else {
		instances, err = existingReplica(ctx, cluster.Kubectl, req.App)
		if err != nil {
			return singleError(err, http.StatusInternalServerError)
		}
	}

	app := models.App{
		AppRef:    req.App,
		Git:       req.Git,
		Route:     req.Route,
		Instances: instances,
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

func existingReplica(ctx context.Context, client *k8s.Clientset, app models.AppRef) (int32, error) {
	// if a deployment exists, use that deployment's replica count
	result, err := client.AppsV1().Deployments(app.Org).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return DefaultInstances, nil
		}
		return 0, err
	}
	return *result.Spec.Replicas, nil
}

func newPipelineRun(uid string, app models.App) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: uid,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Org,
				models.EpinioStageIDLabel:      uid,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "staging",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "staging-triggers-admin",
			PipelineRef:        &v1beta1.PipelineRef{Name: "staging-pipeline"},
			Params: []v1beta1.Param{
				{Name: "APP_NAME", Value: *v1beta1.NewArrayOrString(app.Name)},
				{Name: "ORG", Value: *v1beta1.NewArrayOrString(app.Org)},
				{Name: "ROUTE", Value: *v1beta1.NewArrayOrString(app.Route)},
				{Name: "INSTANCES", Value: *v1beta1.NewArrayOrString(strconv.Itoa(int(app.Instances)))},
				{Name: "IMAGE", Value: *v1beta1.NewArrayOrString(app.ImageURL(gitea.LocalRegistry))},
				{Name: "STAGE_ID", Value: *v1beta1.NewArrayOrString(uid)},
			},
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
							{Name: "url", Value: app.GitURL(deployments.GiteaURL)},
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
	mainDomain, err := domain.MainDomain()
	if err != nil {
		return err
	}

	// Create production certificate if it is provided by user
	// else create a local cluster self-signed tls secret.
	if !strings.Contains(mainDomain, "omg.howdoi.website") {
		err = createCertificate(ctx, cfg, app, mainDomain, "letsencrypt-production")
		if err != nil {
			return errors.Wrap(err, "create production ssl certificate failed")
		}
	} else {
		err = createCertificate(ctx, cfg, app, mainDomain, "selfsigned-issuer")
		if err != nil {
			return errors.Wrap(err, "create local ssl certificate failed")
		}
	}
	return nil
}

func createCertificate(ctx context.Context, cfg *rest.Config, app models.App, systemDomain string, issuer string) error {
	// Notes:
	// - spec.CommonName is length-limited.
	//   At most 64 characters are allowed, as per [RFC 3280](https://www.rfc-editor.org/rfc/rfc3280.txt).
	//   That makes it a problem for long app name and domain combinations.
	// - The spec.dnsNames (SAN, Subject Alternate Names) do not have such a limit.
	// - Luckily CN is deprecated with regard to DNS checking.
	//   The SANs are prefered and usually checked first.
	//
	// As such our solution is to
	// - Keep the full app + domain in the spec.dnsNames/SAN.
	// - Truncate the full app + domain in CN to 64 characters,
	//   replace the tail with an MD5 suffix computed over the
	//   full string as means of keeping the text unique across
	//   apps.

	cn := names.TruncateMD5(fmt.Sprintf("%s.%s", app.Name, systemDomain), 64)
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
        }`, app.Name, app.Org, cn, app.Name, app.Name, systemDomain, issuer)

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
