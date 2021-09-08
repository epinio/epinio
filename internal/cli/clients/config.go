package clients

import (
	"context"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/duration"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// ConfigUpdate updates the credentials stored in the config from the
// currently targeted kube cluster. It does not use the API server.
func (c *EpinioClient) ConfigUpdate(ctx context.Context) error {
	log := c.Log.WithName("ConfigUpdate")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Config", c.Config.Location).
		Msg("Updating the stored credentials from the current cluster")

	details.Info("retrieving credentials")

	user, password, err := getCredentials(ctx, details)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	details.Info("retrieved credentials", "user", user, "password", password)
	details.Info("retrieving server locations")

	api, wss, err := getAPI(ctx, details)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	details.Info("retrieved server locations", "api", api, "wss", wss)
	details.Info("retrieving certs")

	certs, err := getCerts(ctx, details)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	details.Info("retrieved certs", "certs", certs)

	c.Config.User = user
	c.Config.Password = password
	c.Config.API = api
	c.Config.WSS = wss
	c.Config.Certs = certs

	details.Info("saving",
		"user", c.Config.User,
		"pass", c.Config.Password,
		"api", c.Config.API,
		"wss", c.Config.WSS,
		"cert", c.Config.Certs)

	err = c.Config.Save()
	if err != nil {
		c.ui.Exclamation().Msg(errors.Wrap(err, "failed to save configuration").Error())
		return nil
	}

	details.Info("saved")

	c.ui.Success().Msg("Ok")
	return nil
}

func getAPI(ctx context.Context, log logr.Logger) (string, string, error) {
	// This is called only by the admin command `config update`
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

// TODO: https://github.com/epinio/epinio/issues/667
func getCredentials(ctx context.Context, log logr.Logger) (string, string, error) {
	// This is called only by the admin command `config update`
	// which has to talk to the cluster to retrieve the
	// information. This is allowed.

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return "", "", err
	}

	log.Info("got cluster")

	// Waiting for the secret is better than simply trying to get
	// it. This way we automatically handle the case where we try
	// to pull data from a secret still under construction by some
	// other part of the system.
	//
	// See assets/embedded-files/epinio/server.yaml for the
	// definition

	secret, err := cluster.WaitForSecret(ctx,
		deployments.EpinioDeploymentID,
		"epinio-api-auth-data",
		duration.ToServiceSecret(),
	)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get API auth secret")
	}

	log.Info("got secret", "secret", "epinio-api-auth-data")

	user := string(secret.Data["user"])
	pass := string(secret.Data["pass"])

	if user == "" || pass == "" {
		return "", "", errors.New("bad API auth secret, expected fields missing")
	}

	return user, pass, nil
}

func getCerts(ctx context.Context, log logr.Logger) (string, error) {
	// This is called only by the admin command `config update`
	// which has to talk to the cluster to retrieve the
	// information. This is allowed.

	// Save the  CA cert into the config. The regular client
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
		deployments.EpinioDeploymentID,
		deployments.EpinioDeploymentID+"-tls",
		duration.ToServiceSecret(),
	)

	if err != nil {
		return "", errors.Wrap(err, "failed to get API CA cert secret")
	}

	log.Info("got secret", "secret", deployments.EpinioDeploymentID+"-tls")

	return string(secret.Data["ca.crt"]), nil
}
