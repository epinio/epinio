package installer

import (
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

func yamlApply(log logr.Logger, c Component) error {
	path := c.Source.Path
	if len(c.Values) > 0 {
		var err error
		path, err = tmpl(c.String(), c.Source.Path, c.Values)
		if err != nil {
			return err
		}
		defer os.Remove(path)
	}

	args := []string{"apply", "--wait", "--filename", path}

	// Note: providing this namespace will error if the yaml already defines a different one
	if c.Namespace != "" {
		args = append(args, "--namespace", c.Namespace)
	}

	log.Info("run", "args", args)

	message := fmt.Sprintf("applying YAML for '%s' from '%s'", c.ID, c.Source.Path)
	retryErr := retry.Do(
		func() error {
			out, err := helpers.Kubectl(args...)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
			}
			return nil
		},
		retry.RetryIf(func(err error) bool {
			return helpers.Retryable(err.Error())
		}),
		retry.OnRetry(func(n uint, err error) {
			log.V(1).Info("retrying to apply", "path", c.Source.Path)
		}),
		retry.Delay(5*time.Second),
	)
	return retryErr
}

func yamlDelete(log logr.Logger, c Component) error {
	path := c.Source.Path
	if len(c.Values) > 0 {
		var err error
		path, err = tmpl(c.String(), c.Source.Path, c.Values)
		if err != nil {
			return err
		}
		defer os.Remove(path)
	}

	args := []string{"delete", "--wait", "--filename", path}
	if c.Namespace != "" {
		args = append(args, "--namespace", c.Namespace)
	}

	log.Info("run", "args", args)

	message := fmt.Sprintf("deleting YAML for '%s' from '%s'", c.ID, c.Source.Path)
	retryErr := retry.Do(
		func() error {
			out, err := helpers.Kubectl(args...)
			if err != nil {
				if strings.Contains(out, "not found") || strings.Contains(out, "no matches") {
					return nil
				}
				return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
			}
			return nil
		},
		retry.RetryIf(func(err error) bool {
			return helpers.Retryable(err.Error())
		}),
		retry.OnRetry(func(n uint, err error) {
			log.V(1).Info("retrying to delete", "path", c.Source.Path)
		}),
		retry.Delay(5*time.Second),
	)

	return retryErr
}

func tmpl(id string, path string, values Values) (string, error) {
	dat, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tmpl := template.Must(template.New(string(id)).Parse(string(dat)))
	var config strings.Builder
	data := struct {
		Values map[string]string
	}{values.ToMap()}
	if err := tmpl.Execute(&config, data); err != nil {
		return "", err
	}
	tmpfile, err := helpers.CreateTmpFile(config.String())
	if err != nil {
		return "", err
	}
	return tmpfile, nil
}
