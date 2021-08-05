package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling TraefikForwardAuth: " + err.Error())
	}

	message := "Removing helm release " + TraefikForwardAuthDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall traefik-forward-auth --namespace '%s'", TraefikForwardAuthDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", TraefikForwardAuthDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", TraefikForwardAuthDeploymentID, out)
		}
	}

	message = "Deleting TraefikForwardAuth namespace " + TraefikForwardAuthDeploymentID
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
		"kubed-target-namespace":            TraefikForwardAuthDeploymentID,
	}, nil); err != nil {
		return err
	}

	if _, err := c.WaitForSecret(ctx, TraefikForwardAuthDeploymentID, "epinio-ca-root", duration.ToSecretCopied()); err != nil {
		return errors.Wrap(err, "Waiting for epinio CA to be copied")
	}

	if err := k.createOauthClientSecret(ctx, c); err != nil {
		return errors.Wrap(err, "Creating Oauth2 client credentials")
	}
	if err := k.createDeployment(ctx, c, options); err != nil {
		return errors.Wrap(err, "Creating Traefik Forward Auth deployment")
	}

	// createService()
	// createCert() // Not needed? Happens automatically with annotations?
	// createIngress()
	// createMiddleware()

	ui.Success().Msg("TraefikForwardAuth Ingress deployed")

	return nil
}

func (k TraefikForwardAuth) createOauthClientSecret(ctx context.Context, c *kubernetes.Cluster) error {
	passwordAuth, err := auth.RandomPasswordAuth()
	if err != nil {
		return err
	}

	_, err = c.Kubectl.CoreV1().Secrets(TraefikForwardAuthDeploymentID).Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "traefik-forward-auth-client",
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

	c.Kubectl.AppsV1().Deployments(TraefikForwardAuthDeploymentID).Create(ctx, &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "",
			APIVersion: "",
		},
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

	return nil
}

func (k TraefikForwardAuth) GetVersion() string {
	return traefikVersion
}

func (k TraefikForwardAuth) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	ui.Note().KeeplineUnder(1).Msg("Deploying TraefikForwardAuth ...")

	return k.apply(ctx, c, ui, options, false)
}

func (k TraefikForwardAuth) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return errors.New("Not implemented")
}
