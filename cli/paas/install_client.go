package paas

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/deployments"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas/config"
	"github.com/suse/carrier/cli/paas/ui"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultTimeoutSec = 300
)

var deploymentSet = map[string]kubernetes.Deployment{
	"traefik":  &deployments.Traefik{Timeout: DefaultTimeoutSec},
	"quarks":   &deployments.Quarks{Timeout: DefaultTimeoutSec},
	"gitea":    &deployments.Gitea{Timeout: DefaultTimeoutSec},
	"eirini":   &deployments.Eirini{Timeout: DefaultTimeoutSec},
	"registry": &deployments.Registry{Timeout: DefaultTimeoutSec},
	"tekton":   &deployments.Tekton{Timeout: DefaultTimeoutSec},
}

// InstallClient provides functionality for talking to Kubernetes for
// installing Carrier on it.
type InstallClient struct {
	kubeClient *kubernetes.Cluster
	ui         *ui.UI
	config     *config.Config
}

// Install deploys carrier to the cluster.
func (c *InstallClient) Install(cmd *cobra.Command, options *kubernetes.InstallationOptions) error {
	c.ui.Note().Msg("Carrier installing...")

	var err error
	options, err = options.Populate(kubernetes.NewCLIOptionsReader(cmd))
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}

	interactive, err := cmd.Flags().GetBool("interactive")
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}

	if interactive {
		options, err = options.Populate(kubernetes.NewInteractiveOptionsReader(os.Stdout, os.Stdin))
		if err != nil {
			return errors.Wrap(err, "Couldn't install carrier")
		}
	} else {
		options, err = options.Populate(kubernetes.NewDefaultOptionsReader())
		if err != nil {
			return errors.Wrap(err, "Couldn't install carrier")
		}
	}

	c.showInstallConfiguration(options)

	// TODO (post MVP): Run a validation phase which perform
	// additional checks on the values. For example range limits,
	// proper syntax of the string, etc. do it as pghase, and late
	// to report all problems at once, instead of early and
	// piecemal.

	deployment := deploymentSet["traefik"]
	err = deployment.Deploy(c.kubeClient, c.ui, options.ForDeployment(deployment.ID()))
	if err != nil {
		return err
	}

	// Try to give a nip.io domain if the user didn't specify one
	domain, err := options.GetOpt("system_domain", "")
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}

	err = c.fillInMissingSystemDomain(domain)
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}
	if domain.Value.(string) == "" {
		return errors.New("You didn't provide a system_domain and we were unable to setup a nip.io domain (couldn't find and ExternalIP)")
	}

	c.ui.Success().Msg("Created system_domain: " + domain.Value.(string))

	for _, deploymentName := range []string{"quarks", "gitea", "eirini", "registry", "tekton"} {
		deployment := deploymentSet[deploymentName]
		options := options.ForDeployment(deployment.ID())
		err := deployment.Deploy(c.kubeClient, c.ui, options)
		if err != nil {
			return err
		}
	}

	c.ui.Success().Msg("Carrier installed.")
	c.ui.Success().Msg("Your system domain is: " + domain.Value.(string))

	return nil
}

// Uninstall removes carrier from the cluster.
func (c *InstallClient) Uninstall(cmd *cobra.Command) error {
	c.ui.Note().Msg("Carrier uninstalling...")

	deployment := deploymentSet["eirini"]
	err := deployment.Delete(c.kubeClient, c.ui)
	if err != nil {
		return err
	}

	for _, deploymentName := range []string{"tekton", "registry", "gitea", "quarks", "traefik"} {
		d := deploymentSet[deploymentName]
		err := d.Delete(c.kubeClient, c.ui)
		if err != nil {
			return err
		}
	}

	c.ui.Success().Msg("Carrier uninstalled.")

	return nil
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
		if !c.kubeClient.GetPlatform().HasLoadBalancer() {
			ips := c.kubeClient.GetPlatform().ExternalIPs()
			if len(ips) > 0 {
				ip = ips[0]
			}
		} else {
			c.ui.ProgressNote().Msg("Waiting for LoadBalancer IP on traefik service.")
			timeout := time.After(2 * time.Minute)
			ticker := time.Tick(1 * time.Second)
		Exit:
			for {
				select {
				case <-timeout:
					break Exit
				case <-ticker:
					serviceList, err := c.kubeClient.Kubectl.CoreV1().Services("").List(context.Background(), metav1.ListOptions{
						FieldSelector: "metadata.name=traefik",
					})
					if len(serviceList.Items) == 0 {
						return errors.New("Couldn't find the traefik service")
					}
					if err != nil {
						return err
					}
					ingress := serviceList.Items[0].Status.LoadBalancer.Ingress
					if len(ingress) > 0 {
						ip = ingress[0].IP
						break Exit
					}
				}
			}
		}

		if ip != "" {
			domain.Value = fmt.Sprintf("%s.nip.io", ip)
		}
	}

	return nil
}
