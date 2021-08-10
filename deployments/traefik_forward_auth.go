package deployments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/duration"
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type TraefikForwardAuth struct {
	Debug   bool
	Timeout time.Duration
	Log     logr.Logger
}

var _ kubernetes.Deployment = &TraefikForwardAuth{}

const (
	TraefikForwardAuthDeploymentID = "traefik-forward-auth"
	TraefikForwardAuthVersion      = "2.2.0"
)

func (k *TraefikForwardAuth) ID() string {
	return TraefikForwardAuthDeploymentID
}

func (k *TraefikForwardAuth) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *TraefikForwardAuth) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k TraefikForwardAuth) Describe() string {
	return emoji.Sprintf(":cloud:TraefikForwardAuth version: %s", TraefikForwardAuthVersion)
}

func (k TraefikForwardAuth) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing TraefikForwardAuth...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, TraefikForwardAuthDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", TraefikForwardAuthDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping TraefikForwardAuth because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	message := "Deleting TraefikForwardAuth namespace " + TraefikForwardAuthDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, TraefikForwardAuthDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", TraefikForwardAuthDeploymentID)
	}

	ui.Success().Msg("TraefikForwardAuth removed")

	return nil
}

func (k TraefikForwardAuth) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	if err := c.CreateNamespace(ctx, TraefikForwardAuthDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
		"kubed-source-namespace":            CertManagerDeploymentID,
	}, map[string]string{
		"linkerd.io/inject": "enabled",
	}); err != nil {
		return err
	}

	if _, err := c.WaitForSecret(ctx, TraefikForwardAuthDeploymentID, "epinio-ca-root", duration.ToSecretCopied()); err != nil {
		return errors.Wrap(err, "waiting for epinio CA to be copied")
	}

	if err := k.createOauthClientSecret(ctx, c); err != nil {
		return errors.Wrap(err, "creating Oauth2 client credentials")
	}
	if err := k.createDeployment(ctx, c, options); err != nil {
		return errors.Wrap(err, "creating Traefik Forward Auth deployment")
	}

	if err := k.createService(ctx, c); err != nil {
		return errors.Wrap(err, "create Traefik Forward Auth service")
	}

	if err := k.createCertificate(ctx, c, options, ui); err != nil {
		return errors.Wrap(err, "create Traefik Forward Auth service")
	}

	// Wait until the cert is there before we create the Ingress
	// TODO: Fix all the ingresses to wait before they are created
	if _, err := c.WaitForSecret(ctx, TraefikForwardAuthDeploymentID, "traefik-forward-auth-tls", duration.ToSecretCopied()); err != nil {
		return errors.Wrap(err, "waiting for the Trafik forward auth tls certificate to be created")
	}

	if err := k.createIngress(ctx, c, options); err != nil {
		return errors.Wrap(err, "create Traefik Forward Auth ingress")
	}

	if err := k.createMiddleware(ctx, c, options); err != nil {
		return errors.Wrap(err, "create Traefik Forward Auth Middleware")
	}

	ui.Success().Msg("TraefikForwardAuth deployed")

	return nil
}

func (k TraefikForwardAuth) createMiddleware(ctx context.Context, c *kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	client, err := c.ClientMiddleware()
	if err != nil {
		return err
	}

	data := fmt.Sprintf(`{
		"apiVersion": "traefik.containo.us/v1alpha1",
		"kind": "Middleware",
		"metadata": {
			"name": "forward-auth"
		},
		"spec": {
			"forwardAuth": {
				"address": "http://traefik-forward-auth.%s:4181",
				"authResponseHeaders": [
					"X-Forwarded-User"
				]
			}
		}
	}`, TraefikForwardAuthDeploymentID)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	if _, _, err = decoderUnstructured.Decode([]byte(data), nil, obj); err != nil {
		return err
	}

	if _, err = client.Namespace(TraefikForwardAuthDeploymentID).Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (k TraefikForwardAuth) createCertificate(ctx context.Context, c *kubernetes.Cluster, options kubernetes.InstallationOptions, ui *termui.UI) error {
	issuer := options.GetStringNG("tls-issuer")

	// Wait for the cert manager to be present and active. It is required
	if err := waitForCertManagerReady(ctx, ui, c, issuer); err != nil {
		return errors.Wrap(err, "waiting for cert-manager failed")
	}

	domain, err := options.GetString("system_domain", "")
	if err != nil {
		return errors.Wrap(err, "ouldn't get system_domain option")
	}

	// Workaround for cert-manager webhook service not being immediately ready.
	// More here: https://cert-manager.io/v1.2-docs/concepts/webhook/#webhook-connection-problems-shortly-after-cert-manager-installation
	cert := auth.CertParam{
		Name:      TraefikForwardAuthDeploymentID,
		Namespace: TraefikForwardAuthDeploymentID,
		Issuer:    issuer,
		Domain:    domain,
	}
	err = retry.Do(func() error {
		return auth.CreateCertificate(ctx, c, cert, nil)
	},
		retry.RetryIf(func(err error) bool {
			return strings.Contains(err.Error(), " x509: ") ||
				strings.Contains(err.Error(), "failed calling webhook") ||
				strings.Contains(err.Error(), "EOF")
		}),
		retry.OnRetry(func(n uint, err error) {
			ui.Note().Msgf("Retrying creation of %s cert via cert-manager (%d/%d)", TraefikForwardAuthDeploymentID, n, duration.RetryMax)
		}),
		retry.Delay(5*time.Second),
		retry.Attempts(duration.RetryMax),
	)
	if err != nil {
		return errors.Wrapf(err, "failed trying to create the %s server cert", TraefikForwardAuthDeploymentID)
	}

	return nil
}

func (k TraefikForwardAuth) createIngress(ctx context.Context, c *kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	domain, err := options.GetString("system_domain", "")
	if err != nil {
		return err
	}

	pathTypePrefix := networkingv1.PathTypeImplementationSpecific
	subdomain := "auth." + domain

	_, err = c.Kubectl.NetworkingV1().Ingresses(TraefikForwardAuthDeploymentID).Create(
		ctx,
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "traefik-forward-auth",
				Namespace: TraefikForwardAuthDeploymentID,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": "traefik",
					// Traefik v1/v2 tls annotations.
					"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
					"traefik.ingress.kubernetes.io/router.tls":         "true",
					// https://github.com/coderanger/traefik-forward-auth-dex/blob/da8fd51cd49c3c22b4746c9918bb06c9bf7def8b/forward-auth.yml#L61-L63
					// https://github.com/thomseddon/traefik-forward-auth/issues/11#issuecomment-894874141
					"traefik.ingress.kubernetes.io/router.middlewares": TraefikForwardAuthDeploymentID + "-forward-auth@kubernetescrd",
				},
				Labels: map[string]string{
					"app.kubernetes.io/name": "traefik-forward-auth",
				},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: subdomain,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathTypePrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "traefik-forward-auth",
												Port: networkingv1.ServiceBackendPort{
													Number: 4181,
												},
											},
										}}}}}}},
				TLS: []networkingv1.IngressTLS{{
					Hosts:      []string{subdomain},
					SecretName: "traefik-forward-auth-tls",
				}},
			}},
		metav1.CreateOptions{},
	)

	return err
}

func (k TraefikForwardAuth) createService(ctx context.Context, c *kubernetes.Cluster) error {
	_, err := c.Kubectl.CoreV1().Services(TraefikForwardAuthDeploymentID).Create(ctx,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "traefik-forward-auth",
				Labels: map[string]string{
					"app.kubernetes.io/instance": "traefik-forward-auth",
					"app.kubernetes.io/name":     "traefik-forward-auth",
					"app.kubernetes.io/version":  "2.2.0",
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "http",
						Port:     4181,
						Protocol: "TCP",
						TargetPort: intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "http"},
					},
				},
				Selector: map[string]string{
					"app.kubernetes.io/instance": "traefik-forward-auth",
					"app.kubernetes.io/name":     "traefik-forward-auth",
				},
			},
		},
		metav1.CreateOptions{})

	return err
}

func (k TraefikForwardAuth) createOauthClientSecret(ctx context.Context, c *kubernetes.Cluster) error {
	passwordAuth, err := auth.RandomPasswordAuth()
	if err != nil {
		return err
	}

	_, err = c.Kubectl.CoreV1().Secrets(TraefikForwardAuthDeploymentID).Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-client", TraefikForwardAuthDeploymentID),
			},
			StringData: map[string]string{
				"username": passwordAuth.Username,
				"password": passwordAuth.Password,
			},
			Type: "Opaque",
		}, metav1.CreateOptions{})

	return err
}

func (k TraefikForwardAuth) createDeployment(ctx context.Context, c *kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	replicas := int32(1)
	domain, err := options.GetString("system_domain", "")
	if err != nil {
		return err
	}

	cookieSecret, err := randstr.Hex16()
	if err != nil {
		return err
	}

	_, err = c.Kubectl.AppsV1().Deployments(TraefikForwardAuthDeploymentID).Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: TraefikForwardAuthDeploymentID,
			Labels: map[string]string{
				"app.kubernetes.io/instance": TraefikForwardAuthDeploymentID,
				"app.kubernetes.io/name":     TraefikForwardAuthDeploymentID,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/instance": TraefikForwardAuthDeploymentID,
					"app.kubernetes.io/name":     TraefikForwardAuthDeploymentID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: TraefikForwardAuthDeploymentID,
					Labels: map[string]string{
						"app.kubernetes.io/instance": TraefikForwardAuthDeploymentID,
						"app.kubernetes.io/name":     TraefikForwardAuthDeploymentID,
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "epinio-ca-root",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "epinio-ca-root",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "traefik-forward-auth",
							Image: "docker.io/thomseddon/traefik-forward-auth:" + TraefikForwardAuthVersion,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      "TCP",
									ContainerPort: 4181,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "AUTH_HOST",
									Value: "auth." + domain,
								},
								{
									Name:  "COOKIE_DOMAIN",
									Value: domain,
								},
								{
									Name:  "DEFAULT_PROVIDER",
									Value: "oidc",
								},
								{
									Name:  "SECRET",
									Value: cookieSecret,
								},
								{
									Name: "PROVIDERS_OIDC_CLIENT_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "traefik-forward-auth-client",
											},
											Key: "username",
										},
									},
								},
								{
									Name: "PROVIDERS_OIDC_CLIENT_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "traefik-forward-auth-client",
											},
											Key: "password",
										},
									},
								},
								{
									Name:  "PROVIDERS_OIDC_ISSUER_URL",
									Value: fmt.Sprintf("https://%s.%s", DexDeploymentID, domain),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/etc/ssl/certs/epinio-ca-root.crt",
									Name:      "epinio-ca-root",
									SubPath:   "ca.crt",
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   intstr.String,
											StrVal: "http",
										},
									},
								},
								TimeoutSeconds:   1,
								PeriodSeconds:    20,
								SuccessThreshold: 1,
								FailureThreshold: 3,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   intstr.String,
											StrVal: "http",
										},
									},
								},
								TimeoutSeconds:   1,
								PeriodSeconds:    20,
								SuccessThreshold: 1,
								FailureThreshold: 3,
							},
							ImagePullPolicy: "IfNotPresent",
						},
					},
					ServiceAccountName: "",
				},
			},
		},
	}, metav1.CreateOptions{})

	return err
}

func (k TraefikForwardAuth) GetVersion() string {
	return traefikVersion
}

func (k TraefikForwardAuth) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k TraefikForwardAuth) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	ui.Note().KeeplineUnder(1).Msg("Deploying TraefikForwardAuth ...")

	return k.apply(ctx, c, ui, options, false)
}

func (k TraefikForwardAuth) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return errors.New("Not implemented")
}
