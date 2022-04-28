// Package admincmd provides the commands of the admin CLI, which deals with
// installing and configurations
package admincmd

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/helmchart"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const (
	epinioAPIProtocol = "https"
	epinioWSProtocol  = "wss"
	DefaultNamespace  = "workspace"
)

// Admin provides functionality for administering Epinio installations on
// Kubernetes
type Admin struct {
	Settings *settings.Settings
	Log      logr.Logger
	ui       *termui.UI
}

func New() (*Admin, error) {
	settingsSettings, err := settings.Load()
	if err != nil {
		return nil, err
	}

	uiUI := termui.NewUI()

	logger := tracelog.NewLogger().WithName("EpinioSettings").V(3)

	return &Admin{
		ui:       uiUI,
		Settings: settingsSettings,
		Log:      logger,
	}, nil
}

// SettingsUpdate updates the credentials stored in the settings from the
// currently targeted kube cluster. It does not use the API server.
func (a *Admin) SettingsUpdate(ctx context.Context) error {
	log := a.Log.WithName("SettingsUpdate")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	a.ui.Note().
		WithStringValue("Settings", a.Settings.Location).
		Msg("Updating the stored credentials from the current cluster")

	details.Info("retrieving credentials")

	users, err := auth.GetUsersByAge(ctx)
	if len(users) == 0 {
		a.ui.Exclamation().Msg("no user account found")
		return nil
	}
	user := users[0]

	details.Info("retrieved credentials", "user", user.Username, "password", user.Password)
	details.Info("retrieving server locations")

	api, wss, err := getAPI(ctx, details)
	if err != nil {
		a.ui.Exclamation().Msg(err.Error())
		return nil
	}

	details.Info("retrieved server locations", "api", api, "wss", wss)
	details.Info("retrieving certs")

	certs, err := getCerts(ctx, details)
	if err != nil {
		a.ui.Exclamation().Msg(err.Error())
		return nil
	}

	details.Info("retrieved certs", "certs", certs)

	a.Settings.User = user.Username
	a.Settings.Password = user.Password
	a.Settings.API = api
	a.Settings.WSS = wss
	a.Settings.Certs = certs

	details.Info("saving",
		"user", a.Settings.User,
		"pass", a.Settings.Password,
		"api", a.Settings.API,
		"wss", a.Settings.WSS,
		"cert", a.Settings.Certs)

	err = a.Settings.Save()
	if err != nil {
		a.ui.Exclamation().Msg(errors.Wrap(err, "failed to save configuration").Error())
		return nil
	}

	details.Info("saved")

	a.ui.Success().Msg("Ok")
	return nil
}

func getAPI(ctx context.Context, log logr.Logger) (string, string, error) {
	// This is called only by the admin command `settings update`
	// which has to talk to the cluster to retrieve the
	// information. This is allowed.

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return "", "", err
	}

	log.Info("got cluster")

	epinioURL, epinioWsURL, err := getEpinioURL(ctx, cluster)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to resolve epinio api host")
	}

	return epinioURL, epinioWsURL, err
}

func getCerts(ctx context.Context, log logr.Logger) (string, error) {
	// This is called only by the admin command `settings update`
	// which has to talk to the cluster to retrieve the
	// information. This is allowed.

	// Save the  CA cert into the settings. The regular client
	// will then extend the Cert pool with the same, so that it
	// can cerify the server cert.

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return "", err
	}

	log.Info("got cluster")

	// Waiting for the secret is better than simply trying to get
	// it. This way we automatically handle the case where we try
	// to pull data from a secret still under construction by some
	// other part of the system.

	// See the `auth.createCertificate` template for the created
	// Certs, and epinio.go `apply` for the call to
	// `auth.createCertificate`, which determines the secret's
	// name we are using here

	secret, err := cluster.WaitForSecret(ctx,
		helmchart.Namespace(),
		helmchart.EpinioCertificateName+"-tls",
		duration.ToConfigurationSecret(),
	)

	if err != nil {
		return "", errors.Wrap(err, "failed to get API CA cert secret")
	}

	log.Info("got secret", "secret", helmchart.EpinioCertificateName+"-tls")

	return string(secret.Data["tls.crt"]), nil
}

// getEpinioURL finds the URL's for epinio from the cluster
func getEpinioURL(ctx context.Context, cluster *kubernetes.Cluster) (string, string, error) {
	// Get the ingress
	ingresses, err := cluster.ListIngress(ctx, helmchart.Namespace(), "app.kubernetes.io/name=epinio")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to list ingresses for epinio api server")
	}

	if len(ingresses.Items) < 1 {
		return "", "", errors.New("epinio api ingress not found")
	}

	if len(ingresses.Items) > 1 {
		return "", "", errors.New("more than one epinio api ingress found")
	}

	if len(ingresses.Items[0].Spec.Rules) < 1 {
		return "", "", errors.New("epinio api ingress has no rules")
	}

	if len(ingresses.Items[0].Spec.Rules) > 1 {
		return "", "", errors.New("epinio api ingress has more than on rule")
	}

	host := ingresses.Items[0].Spec.Rules[0].Host

	return fmt.Sprintf("%s://%s", epinioAPIProtocol, host), fmt.Sprintf("%s://%s", epinioWSProtocol, host), nil
}
