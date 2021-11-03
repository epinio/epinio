package application

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/names"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// ListRoutes lists all (currently active) routes for the given application
// The list is constructed from the actual Ingresses and not from the stored
// information on the Application Custom Resource.
func ListRoutes(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
	ingressList, err := ingressListForApp(ctx, cluster, appRef)
	if err != nil {
		return []string{}, err
	}

	result := []string{}
	for _, ingress := range ingressList.Items {
		result = append(result, ingress.Spec.Rules[0].Host)
	}

	return result, nil
}

// SyncIngresses ensures that each route in the Application CRD "Routes" field
// has a respective Ingress resource. It also ensures that no other Ingresses
// exist for that application (e.g. for routes that have been removed).
// Returns the current list of routes (after syncing) and error if something goes wrong.
func SyncIngresses(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, username string) ([]string, error) {
	applicationCR, err := Get(ctx, cluster, appRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []string{}, apierror.AppIsNotKnown("cannot sync app ingresses, application resource is missing")
		}
		return []string{}, apierror.InternalError(err, "failed to get the application resource")
	}
	owner := metav1.OwnerReference{
		APIVersion: applicationCR.GetAPIVersion(),
		Kind:       applicationCR.GetKind(),
		Name:       applicationCR.GetName(),
		UID:        applicationCR.GetUID(),
	}

	desiredRoutes, found, err := unstructured.NestedStringSlice(applicationCR.Object, "spec", "routes")
	if !found {
		return []string{}, errors.New("couldn't parse the Application for Routes")
	}
	if err != nil {
		return []string{}, err
	}

	ingressList, err := ingressListForApp(ctx, cluster, appRef)
	if err != nil {
		return []string{}, err
	}

	// Construct a lookup-up map for existing ingresses
	existingIngresses := map[string]networkingv1.Ingress{}
	for _, ingress := range ingressList.Items {
		existingIngresses[ingress.Spec.Rules[0].Host] = ingress
	}

	// Ensure desired routes
	log := tracelog.Logger(ctx)
	desiredRoutesMap := map[string]bool{}
	for _, desiredRoute := range desiredRoutes {
		desiredRoutesMap[desiredRoute] = true
		if _, ok := existingIngresses[desiredRoute]; ok {
			continue
		}
		log.Info("creating app ingress", "org", appRef.Org, "app", appRef.Name, "", desiredRoute)

		ing := newAppIngress(appRef, desiredRoute, username)

		log.Info("app ingress", "name", ing.ObjectMeta.Name)

		ing.SetOwnerReferences([]metav1.OwnerReference{owner})

		// Check if ingress already exists and skip.
		// If it doesn't exist, create the Ingress and the cert for it.
		if _, err := cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).Get(ctx, ing.Name, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				createdIngress, createErr := cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).Create(ctx, ing, metav1.CreateOptions{})
				if createErr != nil {
					return []string{}, errors.Wrap(err, "creating an application Ingress")
				}

				// Create the certificate for this Ingress (Ignores it if it exists)
				cert := auth.CertParam{
					Name:      createdIngress.Name,
					Namespace: appRef.Org,
					Issuer:    viper.GetString("tls-issuer"),
					Domain:    desiredRoute,
					Labels:    map[string]string{"app.kubernetes.io/name": appRef.Name},
				}
				certOwner := &metav1.OwnerReference{
					APIVersion: "networking.k8s.io/v1",
					Kind:       "Ingress",
					Name:       createdIngress.Name,
					UID:        createdIngress.UID,
				}
				log.Info("app cert", "route", cert.Domain, "issuer", cert.Issuer)
				err = auth.CreateCertificate(ctx, cluster, cert, certOwner)
				if err != nil {
					return []string{}, err
				}
			} else if err != nil {
				return []string{}, err
			}
		}
	}

	// Cleanup removed ingresses. Automatically deletes certificates using
	// owner references.
	for route, ingress := range existingIngresses {
		if _, ok := desiredRoutesMap[route]; !ok {
			deletionPropagation := metav1.DeletePropagationBackground
			if err := cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).Delete(ctx, ingress.Name, metav1.DeleteOptions{
				PropagationPolicy: &deletionPropagation,
			}); err != nil {
				return []string{}, err
			}
			log.Info("deleted ingress", ingress.Name)
		}
	}

	return desiredRoutes, nil
}

// newAppIngress is a helper that creates the kube ingress resource for the app
func newAppIngress(appRef models.AppRef, route, username string) *networkingv1.Ingress {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific

	// name is used both for the Ingress name and the secret name
	// We don't create the secret here, we just expect it to be called like that.
	// The caller (SyncIngresses) makes sure the secret is created with the same
	// name and has this Ingress as an owner.
	name := names.IngressName(fmt.Sprintf("%s-%s", appRef.Name, route))
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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
					SecretName: name + "-tls",
				},
			},
		},
	}
}

func ingressListForApp(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*networkingv1.IngressList, error) {
	// Find all user credential secrets
	ingressSelector := labels.Set(map[string]string{
		"app.kubernetes.io/name": appRef.Name,
	}).AsSelector().String()

	return cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).List(ctx, metav1.ListOptions{
		LabelSelector: ingressSelector,
	})
}
