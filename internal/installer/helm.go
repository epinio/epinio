package installer

import (
	"fmt"
	"os"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// TODO helm sdk
func helmUpdate(log logr.Logger, c Component) error {
	currentdir, _ := os.Getwd()
	args := []string{"upgrade", c.Source.Name, "--install", "--namespace", c.Namespace, "--create-namespace", "--wait"}

	if c.Source.IsPath() {
		args = append(args, c.Source.Path)
	} else if c.Source.IsURL() {
		args = append(args, c.Source.URL)
	} else if c.Source.IsHelmRef() {
		args = append(args, "--repo", c.Source.URL)
		if c.Source.Version != "" {
			args = append(args, "--version", c.Source.Version)
		}
		args = append(args, c.Source.Chart)
	} else {
		return errors.New("helm source is incomplete")
	}

	for _, val := range c.Values {
		args = append(args, "--set", fmt.Sprintf("%s=%s", val.Name, val.Value))
	}

	log.Info("run", "args", args)
	if out, err := helpers.RunProc(currentdir, false, "helm", args...); err != nil {
		log.V(1).Info("helm result", "error", err, "out", out)
		return errors.Wrap(err, fmt.Sprintf("failed installing %s, output:\n%s", c.ID, out))
	}

	log.V(1).Info("done")
	return nil
}

func helmUninstall(log logr.Logger, c Component) error {
	currentdir, _ := os.Getwd()
	args := []string{"uninstall", c.Source.Name, "--namespace", c.Namespace, "--wait"}

	log.Info("run", "args", args)
	if out, err := helpers.RunProc(currentdir, false, "helm", args...); err != nil {
		if strings.Contains(out, "not found") {
			return nil
		}

		log.V(1).Info("helm result", "error", err, "out", out)
		return errors.Wrap(err, fmt.Sprintf("failed uninstalling %s, output:\n%s", c.ID, out))
	}

	// TODO we could delete the namespace if it was created by helm

	log.V(1).Info("done")
	return nil
}
