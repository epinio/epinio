package cli

import (
	"fmt"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	skipLinkerdOption = kubernetes.InstallationOption{
		Name:        "skip-linkerd",
		Description: "Assert to epinio that Linkerd is already installed.",
		Type:        kubernetes.BooleanType,
		Default:     false,
		Value:       false,
	}

	emailOption = kubernetes.InstallationOption{
		Name:        "email_address",
		Description: "The email address you are planning to use for getting notifications about your certificates",
		Type:        kubernetes.StringType,
		Default:     "epinio@suse.com",
		Value:       "",
	}

	ingressServiceIPOption = kubernetes.InstallationOption{
		Name:        "loadbalancer-ip",
		Description: "IP address to be assigned to ingress loadbalancer service",
		Type:        kubernetes.StringType,
		Default:     "",
		Value:       "",
	}
)

var neededOptions = kubernetes.InstallationOptions{
	{
		Name:        "skip-traefik",
		Description: "Assert to epinio that there is a Traefik active, even if epinio cannot find it.",
		Type:        kubernetes.BooleanType,
		Default:     false,
		Value:       false,
	},
	skipLinkerdOption,
	{
		Name:        "skip-cert-manager",
		Description: "Assert to epinio that cert-manager is already installed.",
		Type:        kubernetes.BooleanType,
		Default:     false,
		Value:       false,
	},
	{
		Name:        "system_domain",
		Description: "The domain you are planning to use for Epinio. Should be pointing to the traefik public IP (Leave empty to use a omg.howdoi.website domain).",
		Type:        kubernetes.StringType,
		Default:     "",
		Value:       "",
	},
	emailOption,
	{
		Name:        "user",
		Description: "The user name for authenticating all API requests",
		Type:        kubernetes.StringType,
		Default:     "",
		Value:       "",
		DynDefaultFunc: func(o *kubernetes.InstallationOption) error {
			uid, err := randstr.Hex16()
			if err != nil {
				return err
			}
			o.Value = uid
			return nil
		},
	},
	{
		Name:        "password",
		Description: "The password for authenticating all API requests",
		Type:        kubernetes.StringType,
		Default:     "",
		Value:       "",
		DynDefaultFunc: func(o *kubernetes.InstallationOption) error {
			uid, err := randstr.Hex16()
			if err != nil {
				return err
			}
			o.Value = uid
			return nil
		},
	},
	{
		Name: "tls-issuer",
		Description: fmt.Sprintf("The name of the cluster issuer to use. Epinio creates three options: '%s', '%s', and '%s'.",
			deployments.EpinioCAIssuer,
			deployments.LetsencryptIssuer,
			deployments.SelfSignedIssuer),
		Type:    kubernetes.StringType,
		Default: deployments.EpinioCAIssuer,
		Value:   deployments.EpinioCAIssuer,
	},
	{
		Name:        "use-internal-registry-node-port",
		Description: "Make the internal registry accessible via a node port, so kubelet can access the registry without trusting its cert.",
		Type:        kubernetes.BooleanType,
		Default:     true,
		Value:       true,
	},
	ingressServiceIPOption,
}

var traefikOptions = kubernetes.InstallationOptions{skipLinkerdOption, ingressServiceIPOption}

var certManagerOptions = kubernetes.InstallationOptions{emailOption}

const (
	DefaultOrganization = "workspace"
)

func init() {
	CmdInstall.Flags().BoolP("interactive", "i", false, "Whether to ask the user or not (default not)")
	CmdInstall.Flags().BoolP("skip-default-org", "s", false, "Set this to skip creating a default org")

	CmdInstallIngress.Flags().BoolP("interactive", "i", false, "Whether to ask the user or not (default not)")

	CmdInstallCertManager.Flags().BoolP("interactive", "i", false, "Whether to ask the user or not (default not)")

	neededOptions.AsCobraFlagsFor(CmdInstall.Flags())
	traefikOptions.AsCobraFlagsFor(CmdInstallIngress.Flags())
	certManagerOptions.AsCobraFlagsFor(CmdInstallCertManager.Flags())
}

// CmdInstall implements the command: epinio install
var CmdInstall = &cobra.Command{
	Use:   "install",
	Short: "install Epinio in your configured kubernetes cluster",
	Long:  `install Epinio PaaS in your configured kubernetes cluster`,
	Args:  cobra.ExactArgs(0),
	RunE:  install,
}

// Note: The command is called `install-ingress` instead of `install
// ingress` because we already have `install` as a command, and Cobra
// does not allow such overlapped definitions. While we could convert
// the existing `install` command into an ensemble and then make the
// existing functionality available as `install <something>` it makes
// the quick install more verbose. So, for now the new functionality
// is exposed through a new toplevel command.

// CmdInstallIngress implements the command: epinio install-ingress
var CmdInstallIngress = &cobra.Command{
	Use:   "install-ingress",
	Short: "install Epinio's Ingress in your configured kubernetes cluster",
	Long:  `install Epinio Ingress controller in your configured kubernetes cluster`,
	Args:  cobra.ExactArgs(0),
	RunE:  installIngress,
}

// CmdInstallCertManager implements the command: epinio install-cert-manager
var CmdInstallCertManager = &cobra.Command{
	Use:   "install-cert-manager",
	Short: "install Epinio's cert-manager in your configured kubernetes cluster",
	Long:  `install Epinio cert-manager controller in your configured kubernetes cluster`,
	Args:  cobra.ExactArgs(0),
	RunE:  installCertManager,
}

// install is the backend for CmdInstall.
// It adds epinio to the targeted cluster
func install(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	skipTraefik, err := cmd.Flags().GetBool("skip-traefik")
	if err != nil {
		return errors.Wrap(err, "could not read option --skip-traefik")
	}

	ingressIP, err := cmd.Flags().GetString("loadbalancer-ip")
	if err != nil {
		return errors.Wrap(err, "could not read option --loadbalancer-ip")
	}

	if ingressIP != "" && skipTraefik {
		return errors.New("cannot have --skip-traefik and --loadbalancer-ip together")
	}

	installClient, installCleanup, err := clients.NewInstallClient(cmd.Context(), &neededOptions)
	defer func() {
		if installCleanup != nil {
			installCleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.Install(cmd.Context(), cmd.Flags())
	if err != nil {
		return errors.Wrap(err, "error installing Epinio")
	}

	// Installation complete. Run `org create`, and `target`.

	epinioClient, err := clients.NewEpinioClient(cmd.Context())
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	// Post Installation Tasks:
	// - Retrieve API certs and credentials, save to configuration
	//
	// - Create and target a default organization, so that the
	//   user can immediately begin to push applications.
	//
	// Dev Note: The targeting is done to ensure that a epinio
	// config left over from a previous installation will contain
	// a valid organization. Without it may contain the name of a
	// now invalid organization from said previous install. This
	// then breaks push and other commands in non-obvious ways.

	err = epinioClient.ConfigUpdate(cmd.Context())
	if err != nil {
		return errors.Wrap(err, "error updating config")
	}

	skipDefaultOrg, err := cmd.Flags().GetBool("skip-default-org")
	if err != nil {
		return errors.Wrap(err, "error reading option --skip-default-org")
	}

	if !skipDefaultOrg {
		err := epinioClient.CreateOrg(DefaultOrganization)

		if err != nil {
			return errors.Wrap(err, "error creating org")
		}

		err = epinioClient.Target(DefaultOrganization)
		if err != nil {
			return errors.Wrap(err, "failed to set target")
		}
	}

	return nil
}

// installIngress is the backend for CmdInstallIngress.
// It adds epinio's ingress controller to the targeted cluster
func installIngress(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	installClient, installCleanup, err := clients.NewInstallClient(cmd.Context(), &traefikOptions)
	defer func() {
		if installCleanup != nil {
			installCleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.InstallIngress(cmd)
	if err != nil {
		return errors.Wrap(err, "error installing Epinio ingress")
	}

	return nil
}

// installCertManager is the backend for CmdInstallCertManager.
// It adds epinio's cert manager to the targeted cluster
func installCertManager(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	installClient, installCleanup, err := clients.NewInstallClient(cmd.Context(), &certManagerOptions)
	defer func() {
		if installCleanup != nil {
			installCleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.InstallCertManager(cmd)
	if err != nil {
		return errors.Wrap(err, "error installing Epinio cert-manager")
	}

	return nil
}
