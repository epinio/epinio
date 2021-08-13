package deployments

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GoogleServices struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &GoogleServices{}

const (
	GoogleServicesDeploymentID = "google-service-broker"
	googleServicesVersion      = "0.1.0"
	googleServicesChartFile    = "gcp-service-broker-0.1.0.tgz"
)

func (k GoogleServices) ID() string {
	return GoogleServicesDeploymentID
}

func (k GoogleServices) Describe() string {
	return emoji.Sprintf(":cloud:GoogleServices version: %s\n", googleServicesVersion)
}

func (k GoogleServices) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k GoogleServices) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	err := c.WaitForNamespaceMissing(ctx, ui, GoogleServicesDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("GoogleServices removed")

	return nil
}

// Delete removes GoogleServices from kubernetes cluster
func (k GoogleServices) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing GoogleServices...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, GoogleServicesDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", GoogleServicesDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping GoogleServices because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling GoogleServices: " + err.Error())
	}

	message := "Removing helm release " + GoogleServicesDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.RunProc(currentdir, k.Debug,
				"helm", "uninstall", GoogleServicesDeploymentID, "--namespace", GoogleServicesDeploymentID)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", GoogleServicesDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", GoogleServicesDeploymentID, out)
		}
	}

	message = "Deleting GoogleServices namespace " + GoogleServicesDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, GoogleServicesDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", GoogleServicesDeploymentID)
	}

	return nil
}

func (k GoogleServices) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, GoogleServicesDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	tarPath, err := helpers.ExtractFile(googleServicesChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	serviceAccountJSONPath, err := options.GetString("service-account-json", GoogleServicesDeploymentID)
	if err != nil {
		return errors.Wrap(err, "failed to read service-account-json value")
	}
	if _, err := os.Stat(serviceAccountJSONPath); os.IsNotExist(err) {
		return errors.Errorf("%s file does not exist", serviceAccountJSONPath)
	}
	jsonContent, err := ioutil.ReadFile(serviceAccountJSONPath)
	if err != nil {
		return err
	}
	tmpDir, err := ioutil.TempDir("", "google-service-broker-values")
	if err != nil {
		return errors.Wrap(err, "can't create temp directory")
	}
	defer os.RemoveAll(tmpDir)

	valuesYamlPath := path.Join(tmpDir, "values.yaml")
	valuesYaml := fmt.Sprintf(`
broker:
  service_account_json: '%s'`, strings.Replace(string(jsonContent), "\n", "", -1))
	err = ioutil.WriteFile(valuesYamlPath, []byte(valuesYaml), 0644)
	if err != nil {
		return err
	}

	if out, err := helpers.RunProc(currentdir, k.Debug,
		"helm", action, GoogleServicesDeploymentID, "--namespace", GoogleServicesDeploymentID, "--values", valuesYamlPath, tarPath); err != nil {
		return errors.New("Failed installing GoogleServices: " + out)
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, GoogleServicesDeploymentID, "app=google-service-broker-mysql", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting GoogleServices database to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, GoogleServicesDeploymentID, "app=google-service-broker-mysql", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting GoogleServices database to be running")
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, GoogleServicesDeploymentID, "app.kubernetes.io/name=gcp-service-broker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting GoogleServices to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, GoogleServicesDeploymentID, "app.kubernetes.io/name=gcp-service-broker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting GoogleServices to be running")
	}

	ui.Success().Msg("GoogleServices deployed")

	return nil
}

func (k GoogleServices) GetVersion() string {
	return googleServicesVersion
}

func (k GoogleServices) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, GoogleServicesDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", MinibrokerDeploymentID)
	}
	if existsAndOwned {
		ui.Exclamation().Msg("GoogleServices already installed, skipping")
		return nil
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying GoogleServices...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k GoogleServices) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		GoogleServicesDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + GoogleServicesDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading GoogleServices...")

	return k.apply(ctx, c, ui, options, true)
}
