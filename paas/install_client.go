package paas

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/helpers"
	"github.com/suse/carrier/kubernetes"
	kubeconfig "github.com/suse/carrier/kubernetes/config"
	"github.com/suse/carrier/paas/config"
	"github.com/suse/carrier/paas/ui"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultTimeoutSec = 300
)

// InstallClient provides functionality for talking to Kubernetes for
// installing Carrier on it.
type InstallClient struct {
	kubeClient *kubernetes.Cluster
	options    *kubernetes.InstallationOptions
	ui         *ui.UI
	config     *config.Config
	Log        logr.Logger
}

func NewInstallClient(flags *pflag.FlagSet, options *kubernetes.InstallationOptions) (*InstallClient, func(), error) {
	restConfig, err := kubeconfig.KubeConfig()
	if err != nil {
		return nil, nil, err
	}
	cluster, err := kubernetes.NewClusterFromClient(restConfig)
	if err != nil {
		return nil, nil, err
	}
	uiUI := ui.NewUI()
	configConfig, err := config.Load(flags)
	if err != nil {
		return nil, nil, err
	}
	logger := kubeconfig.NewInstallClientLogger()
	installClient := &InstallClient{
		kubeClient: cluster,
		ui:         uiUI,
		config:     configConfig,
		Log:        logger,
		options:    options,
	}
	return installClient, func() {
	}, nil
}

// Install deploys carrier to the cluster.
func (c *InstallClient) Install(cmd *cobra.Command) error {
	log := c.Log.WithName("Install")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Carrier installing...")

	var err error
	details.Info("process cli options")
	c.options, err = c.options.Populate(kubernetes.NewCLIOptionsReader(cmd))
	if err != nil {
		return err
	}

	interactive, err := cmd.Flags().GetBool("interactive")
	if err != nil {
		return err
	}

	if interactive {
		details.Info("query user for options")
		c.options, err = c.options.Populate(kubernetes.NewInteractiveOptionsReader(os.Stdout, os.Stdin))
		if err != nil {
			return err
		}
	} else {
		details.Info("fill defaults into options")
		c.options, err = c.options.Populate(kubernetes.NewDefaultOptionsReader())
		if err != nil {
			return err
		}
	}

	details.Info("show option configuration")
	c.showInstallConfiguration(c.options)

	// TODO (post MVP): Run a validation phase which perform
	// additional checks on the values. For example range limits,
	// proper syntax of the string, etc. do it as pghase, and late
	// to report all problems at once, instead of early and
	// piecemal.

	if err := c.InstallDeployment(&deployments.Traefik{Timeout: DefaultTimeoutSec}, details); err != nil {
		return err
	}

	// Try to give a omg.howdoi.website domain if the user didn't specify one
	domain, err := c.options.GetOpt("system_domain", "")
	if err != nil {
		return err
	}

	details.Info("ensure system-domain")
	err = c.fillInMissingSystemDomain(domain)
	if err != nil {
		return err
	}
	if domain.Value.(string) == "" {
		return errors.New("You didn't provide a system_domain and we were unable to setup a omg.howdoi.website domain (couldn't find an ExternalIP)")
	}

	c.ui.Success().Msg("Created system_domain: " + domain.Value.(string))

	for _, deployment := range []kubernetes.Deployment{
		&deployments.Quarks{Timeout: DefaultTimeoutSec},
		&deployments.Workloads{Timeout: DefaultTimeoutSec},
		&deployments.Gitea{Timeout: DefaultTimeoutSec},
		&deployments.Registry{Timeout: DefaultTimeoutSec},
		&deployments.Tekton{Timeout: DefaultTimeoutSec},
		&deployments.ServiceCatalog{Timeout: DefaultTimeoutSec},
	} {
		if err := c.InstallDeployment(deployment, details); err != nil {
			return err
		}
	}

	c.ui.Success().WithStringValue("System domain", domain.Value.(string)).Msg("Carrier installed.")

	return nil
}

// Uninstall removes carrier from the cluster.
func (c *InstallClient) Uninstall(cmd *cobra.Command) error {
	log := c.Log.WithName("Uninstall")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Carrier uninstalling...")

	for _, deployment := range []kubernetes.Deployment{
		&deployments.Minibroker{Timeout: DefaultTimeoutSec},
		&deployments.GoogleServices{Timeout: DefaultTimeoutSec},
		&deployments.ServiceCatalog{Timeout: DefaultTimeoutSec},
		&deployments.Workloads{Timeout: DefaultTimeoutSec},
		&deployments.Tekton{Timeout: DefaultTimeoutSec},
		&deployments.Registry{Timeout: DefaultTimeoutSec},
		&deployments.Gitea{Timeout: DefaultTimeoutSec},
		&deployments.Quarks{Timeout: DefaultTimeoutSec},
		&deployments.Traefik{Timeout: DefaultTimeoutSec},
	} {
		if err := c.UninstallDeployment(deployment, details); err != nil {
			return err
		}
	}

	c.ui.Success().Msg("Carrier uninstalled.")

	return nil
}

// InstallDeployment installs one single Deployment on the cluster
func (c *InstallClient) InstallDeployment(deployment kubernetes.Deployment, logger logr.Logger) error {
	logger.Info("deploy", "Deployment", deployment.ID())
	return deployment.Deploy(c.kubeClient, c.ui, c.options.ForDeployment(deployment.ID()))
}

// UninstallDeployment uninstalls one single Deployment from the cluster
func (c *InstallClient) UninstallDeployment(deployment kubernetes.Deployment, logger logr.Logger) error {
	logger.Info("remove", "Deployment", deployment.ID())
	return deployment.Delete(c.kubeClient, c.ui)
}

// showInstallConfiguration prints the options and their values to stdout, to
// inform the user of the detected and chosen configuration
func (c *InstallClient) showInstallConfiguration(opts *kubernetes.InstallationOptions) {
	m := c.ui.Normal()
	for _, opt := range *opts {
		name := "  :compass: " + opt.Name
		switch opt.Type {
		case kubernetes.BooleanType:
			m = m.WithBoolValue(name, opt.Value.(bool))
		case kubernetes.StringType:
			m = m.WithStringValue(name, opt.Value.(string))
		case kubernetes.IntType:
			m = m.WithIntValue(name, opt.Value.(int))
		}
	}
	m.Msg("Configuration...")
}

func (c *InstallClient) fillInMissingSystemDomain(domain *kubernetes.InstallationOption) error {
	if domain.Value.(string) == "" {
		ip := ""
		s := c.ui.Progressf("Waiting for LoadBalancer IP on traefik service.")
		defer s.Stop()
		err := helpers.RunToSuccessWithTimeout(
			func() error {
				return c.fetchIP(&ip)
			}, time.Duration(2)*time.Minute, 3*time.Second)
		if err != nil {
			if strings.Contains(err.Error(), "Timed out after") {
				return errors.New("Timed out waiting for LoadBalancer IP on traefik service.\n" +
					"Ensure your kubernetes platform has the ability to provision LoadBalancer IP address.\n\n" +
					"Follow these steps to enable this ability\n" +
					"https://github.com/SUSE/carrier/blob/main/docs/install.md")
			}
			return err
		}

		if ip != "" {
			domain.Value = fmt.Sprintf("%s.omg.howdoi.website", ip)
		}
	}

	return nil
}

func (c *InstallClient) fetchIP(ip *string) error {
	serviceList, err := c.kubeClient.Kubectl.CoreV1().Services("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "metadata.name=traefik",
	})
	if len(serviceList.Items) == 0 {
		return errors.New("couldn't find the traefik service")
	}
	if err != nil {
		return err
	}
	ingress := serviceList.Items[0].Status.LoadBalancer.Ingress
	if len(ingress) <= 0 {
		return errors.New("ingress list is empty in traefik service")
	}
	*ip = ingress[0].IP

	return nil
}
