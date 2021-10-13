package application

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
)

const (
	DefaultInstances = int32(1)
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

	route, err := domain.AppDefaultRoute(ctx, req.App.Name)
	if err != nil {
		return apierror.InternalError(err)
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

	log.Info("deploying app ingress", "org", org, "app", req.App, "", route)

	ing := newAppIngress(req.App, route, username)

	log.Info("app ingress", "name", ing.ObjectMeta.Name)

	ing.SetOwnerReferences([]metav1.OwnerReference{owner})
	if _, err := cluster.Kubectl.NetworkingV1().Ingresses(req.App.Org).Create(ctx, ing, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if _, err := cluster.Kubectl.NetworkingV1().Ingresses(req.App.Org).Update(ctx, ing, metav1.UpdateOptions{}); err != nil {
				return apierror.InternalError(err)
			}
		} else {
			return apierror.InternalError(err)
		}
	}

	// Delete previous pipelineruns except for the current one
	if req.Stage.ID != "" {
		if err := application.Unstage(ctx, cluster, req.App, req.Stage.ID); err != nil {
			return apierror.InternalError(err)
		}
	}

	resp := models.DeployResponse{
		Route: route,
	}
	err = response.JSON(c, resp)
	if err != nil {
		return apierror.InternalError(err)
	}

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

// newAppIngress is a helper that creates the kube ingress resource for the app
func newAppIngress(appRef models.AppRef, route, username string) *networkingv1.Ingress {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.IngressName(appRef.Name),
			Annotations: map[string]string{
				"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
				"traefik.ingress.kubernetes.io/router.tls":         "true",
				"kubernetes.io/ingress.class":                      "traefik",
			},
			Labels: map[string]string{
				"app.kubernetes.io/component":  "application",
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/name":       appRef.Name,
				"app.kubernetes.io/created-by": username,
				"app.kubernetes.io/part-of":    appRef.Org,
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: route,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: names.ServiceName(appRef.Name),
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
									Path:     "/",
									PathType: &pathTypeImplementationSpecific,
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{
						route,
					},
					SecretName: fmt.Sprintf("%s-tls", appRef.Name),
				},
			},
		},
	}
}
