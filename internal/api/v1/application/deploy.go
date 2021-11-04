package application

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/names"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const (
	DefaultInstances = int32(1)
	LocalRegistry    = "127.0.0.1:30500/apps"
)

type deployParam struct {
	models.AppRef
	ImageURL    string
	Username    string
	Instances   int32
	Stage       models.StageRef
	Owner       metav1.OwnerReference
	Environment models.EnvVariableList
	Services    application.AppServiceBindList
}

// Deploy handles the API endpoint /orgs/:org/applications/:app/deploy
// It creates the deployment, service and ingress (kube) resources for the app
func (hc Controller) Deploy(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	org := c.Param("org")
	name := c.Param("app")
	username := requestctx.User(ctx)

	req := models.DeployRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequest("Failed to unmarshal deploy request ", err.Error())
	}

	if name != req.App.Name {
		return apierror.NewBadRequest("name parameter from URL does not match name param in body")
	}
	if org != req.App.Org {
		return apierror.NewBadRequest("org parameter from URL does not match org param in body")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	// check application resource
	applicationCR, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown("cannot deploy app, application resource is missing")
		}
		return apierror.InternalError(err, "failed to get the application resource")
	}
	owner := metav1.OwnerReference{
		APIVersion: applicationCR.GetAPIVersion(),
		Kind:       applicationCR.GetKind(),
		Name:       applicationCR.GetName(),
		UID:        applicationCR.GetUID(),
	}

	// determine number of desired instances
	instances, err := application.Scaling(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to access application's desired instances")
	}

	// determine runtime environment, if any
	environment, err := application.Environment(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to access application's runtime environment")
	}

	// determine bound services, if any
	services, err := application.BoundServices(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to access application's bound services")
	}

	bindings, err := application.ToBinds(ctx, services, req.App.Name, username)
	if err != nil {
		return apierror.InternalError(err, "failed to process application's bound services")
	}

	deployParams := deployParam{
		AppRef:      req.App,
		Owner:       owner,
		Environment: environment.List(),
		Services:    bindings,
		Instances:   instances,
		ImageURL:    req.ImageURL,
		Username:    username,
	}

	log.Info("deploying app", "org", org, "app", req.App)

	deployParams.ImageURL, err = replaceInternalRegistry(ctx, deployParams.ImageURL)
	if err != nil {
		return apierror.InternalError(err, "preparing ImageURL registry for use by Kubernetes")
	}

	deployment := newAppDeployment(req.Stage.ID, deployParams)
	deployment.SetOwnerReferences([]metav1.OwnerReference{owner})
	if _, err := cluster.Kubectl.AppsV1().Deployments(req.App.Org).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if _, err := cluster.Kubectl.AppsV1().Deployments(req.App.Org).Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
				return apierror.InternalError(err)
			}
		} else {
			return apierror.InternalError(err)
		}
	}

	log.Info("deploying app service", "org", org, "app", req.App)

	svc := newAppService(req.App, username)

	log.Info("app service", "name", svc.ObjectMeta.Name)

	svc.SetOwnerReferences([]metav1.OwnerReference{owner})
	if _, err := cluster.Kubectl.CoreV1().Services(req.App.Org).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			service, err := cluster.Kubectl.CoreV1().Services(req.App.Org).Get(ctx, svc.Name, metav1.GetOptions{})
			if err != nil {
				return apierror.InternalError(err)
			}

			svc.ResourceVersion = service.ResourceVersion
			svc.Spec.ClusterIP = service.Spec.ClusterIP
			if _, err := cluster.Kubectl.CoreV1().Services(req.App.Org).Update(ctx, svc, metav1.UpdateOptions{}); err != nil {
				return apierror.InternalError(err)
			}
		} else {
			return apierror.InternalError(err)
		}
	}

	routes, err := application.SyncIngresses(ctx, cluster, req.App, username)
	if err != nil {
		return apierror.InternalError(err, "syncing application Ingresses")
	}

	// Delete previous pipelineruns except for the current one
	if req.Stage.ID != "" {
		if err := application.Unstage(ctx, cluster, req.App, req.Stage.ID); err != nil {
			return apierror.InternalError(err)
		}
	}

	response.OKReturn(c, models.DeployResponse{
		Routes: routes,
	})
	return nil
}

// newAppDeployment is a helper that creates the kube deployment resource for the app
func newAppDeployment(stageID string, deployParams deployParam) *appsv1.Deployment {
	automountServiceAccountToken := true
	labels := map[string]string{
		"app.kubernetes.io/name":       deployParams.Name,
		"app.kubernetes.io/part-of":    deployParams.Org,
		"app.kubernetes.io/component":  "application",
		"app.kubernetes.io/managed-by": "epinio",
		"app.kubernetes.io/created-by": deployParams.Username,
	}
	if stageID != "" {
		labels["epinio.suse.org/stage-id"] = stageID
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deployParams.Name,
			Labels: map[string]string{
				"app.kubernetes.io/name":       deployParams.Name,
				"app.kubernetes.io/part-of":    deployParams.Org,
				"app.kubernetes.io/component":  "application",
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/created-by": deployParams.Username,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &deployParams.Instances,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": deployParams.Name,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"app.kubernetes.io/name": deployParams.Name,
					},
				},
				Spec: v1.PodSpec{
					ServiceAccountName:           deployParams.Org,
					AutomountServiceAccountToken: &automountServiceAccountToken,
					Volumes:                      deployParams.Services.ToVolumesArray(),
					Containers: []v1.Container{
						{
							Name:  deployParams.Name,
							Image: deployParams.ImageURL,
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 8080,
								},
							},
							Env:          deployParams.Environment.ToEnvVarArray(deployParams.AppRef),
							VolumeMounts: deployParams.Services.ToMountsArray(),
						},
					},
				},
			},
		},
	}
}

// newAppService is a helper that creates the kube service resource for the app
func newAppService(app models.AppRef, username string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.ServiceName(app.Name),
			Namespace: app.Org,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                      "traefik",
				"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
				"traefik.ingress.kubernetes.io/router.tls":         "true",
			},
			Labels: map[string]string{
				"app.kubernetes.io/component":  "application",
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Org,
				"app.kubernetes.io/created-by": username,
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port:       8080,
					Protocol:   v1.ProtocolTCP,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/component": "application",
				"app.kubernetes.io/name":      app.Name,
			},
			Type: v1.ServiceTypeClusterIP,
		},
	}
}

// replaceInternalRegistry replaces the registry part of ImageURL with the localhost
// version of the internal Epinio registry.
// That only happens if we are deploying an image from the Epinio registry
// and that registry doesn't have a certificate signed by a well-known CA.
// Otherwise leave the ImageURL as is because either:
// - the Epinio registry is deployed on Kubernetes with a valid cert (e.g. letsencrypt)
// - the Epinio registry is an external one (if Epinio was deployed that way)
// - a pre-existing image is being deployed (coming from an outer registry, not ours)
// TODO: Also check that we don't use an external registry? the flag below is not compatible with
// the external registry ones, validate on installation?
func replaceInternalRegistry(ctx context.Context, imageURL string) (string, error) {
	registryPublicURL, err := domain.EpinioRegistryPublicURL(ctx)
	if err != nil {
		return imageURL, err
	}
	imageURLParts := strings.Split(imageURL, "/")
	imageRegistry := strings.Join(imageURLParts[0:len(imageURLParts)-1], "/") // All but the last part
	image := imageURLParts[len(imageURLParts)-1]                              // The last part

	result := imageURL
	if imageRegistry == registryPublicURL && viper.GetBool("use-internal-registry-node-port") {
		result = fmt.Sprintf("%s/%s", LocalRegistry, image)
	}

	return result, nil
}
