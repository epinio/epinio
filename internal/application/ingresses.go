package application

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/names"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
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
		result = append(result, "https://"+ingress.Spec.Rules[0].Host)
	}

	return result, nil
}

// SyncIngresses ensures that each domain in the Application CRD "Domains" field
// has a respective Ingress resource. It also ensures that no other Ingresses
// exist for that application (e.g. domains that have been removed).
// Returns the current list of domains (after syncing) and error if something goes wrong.
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

	desiredDomains, found, err := unstructured.NestedStringSlice(applicationCR.Object, "spec", "domains")
	if !found {
		return []string{}, errors.New("couldn't parse the Application for Domains")
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

	// Ensure desired domains
	log := tracelog.Logger(ctx)
	desiredDomainsMap := map[string]bool{}
	for _, desiredDomain := range desiredDomains {
		desiredDomainsMap[desiredDomain] = true
		if _, ok := existingIngresses[desiredDomain]; ok {
			continue
		}
		log.Info("creating app ingress", "org", appRef.Org, "app", appRef.Name, "", desiredDomain)

		ingressPrefix, err := randstr.Hex16()
		if err != nil {
			return []string{}, err
		}

		ing := newAppIngress(appRef, desiredDomain, username, ingressPrefix)

		log.Info("app ingress", "name", ing.ObjectMeta.Name)

		// TODO: All Ingresses have the same name and we simply overwrite the same
		// Ingress again and again! We need to generate unique names for the ingresses.
		ing.SetOwnerReferences([]metav1.OwnerReference{owner})
		if _, err := cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).Create(ctx, ing, metav1.CreateOptions{}); err != nil {
			if apierrors.IsAlreadyExists(err) {
				if _, err := cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).Update(ctx, ing, metav1.UpdateOptions{}); err != nil {
					return []string{}, err
				}
			} else {
				return []string{}, err
			}
		}
	}

	// Cleanup removed ingresses
	for domain, ingress := range existingIngresses {
		if _, ok := desiredDomainsMap[domain]; !ok {
			if err := cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).Delete(ctx, ingress.Name, metav1.DeleteOptions{}); err != nil {
				return []string{}, err
			}
			log.Info("deleted ingress", ingress.Name)
		}
	}

	return desiredDomains, nil
}

// newAppIngress is a helper that creates the kube ingress resource for the app
func newAppIngress(appRef models.AppRef, route, username, prefix string) *networkingv1.Ingress {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{

			Name: names.IngressName(fmt.Sprintf("%s-%s", prefix, appRef.Name)),
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

func ingressListForApp(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*networkingv1.IngressList, error) {
	// Find all user credential secrets
	ingressSelector := labels.Set(map[string]string{
		"app.kubernetes.io/name": appRef.Name,
	}).AsSelector().String()

	return cluster.Kubectl.NetworkingV1().Ingresses(appRef.Org).List(ctx, metav1.ListOptions{
		LabelSelector: ingressSelector,
	})
}
