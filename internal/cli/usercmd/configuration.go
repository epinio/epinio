package usercmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Configurations gets all Epinio configurations in the targeted namespace
func (c *EpinioClient) Configurations(all bool) error {
	log := c.Log.WithName("Configurations").WithValues("Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	msg := c.ui.Note()
	if all {
		msg.Msg("Listing all configurations")
	} else {
		msg.
			WithStringValue("Namespace", c.Settings.Namespace).
			Msg("Listing configurations")

		if err := c.TargetOk(); err != nil {
			return err
		}
	}

	details.Info("list configurations")

	var configurations models.ConfigurationResponseList
	var err error

	if all {
		configurations, err = c.API.AllConfigurations()
	} else {
		configurations, err = c.API.Configurations(c.Settings.Namespace)
	}
	if err != nil {
		return err
	}

	details.Info("list configurations")

	sort.Sort(configurations)

	details.Info("show configurations")

	msg = c.ui.Success()
	if all {
		msg = msg.WithTable("Namespace", "Name", "Created", "Type", "Origin", "Applications")

		for _, configuration := range configurations {
			msg = msg.WithTableRow(
				configuration.Meta.Namespace,
				configuration.Meta.Name,
				configuration.Meta.CreatedAt.String(),
				configuration.Configuration.Type,
				configuration.Configuration.Origin,
				strings.Join(configuration.Configuration.BoundApps, ", "))
		}
	} else {
		msg = msg.WithTable("Name", "Created", "Type", "Origin", "Applications")

		for _, configuration := range configurations {
			msg = msg.WithTableRow(
				configuration.Meta.Name,
				configuration.Meta.CreatedAt.String(),
				configuration.Configuration.Type,
				configuration.Configuration.Origin,
				strings.Join(configuration.Configuration.BoundApps, ", "))
		}
	}

	msg.Msg("Epinio Configurations:")

	return nil
}

// ConfigurationMatching returns all Epinio configurations having the specified prefix
// in their name.
func (c *EpinioClient) ConfigurationMatching(ctx context.Context, prefix string) []string {
	log := c.Log.WithName("ConfigurationMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	resp, err := c.API.ConfigurationMatch(c.Settings.Namespace, prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	sort.Strings(result)

	log.Info("matches", "found", result)
	return result
}

// BindConfiguration attaches a configuration specified by name to the named application,
// both in the targeted namespace.
func (c *EpinioClient) BindConfiguration(configurationName, appName string) error {
	log := c.Log.WithName("Bind Configuration To Application").
		WithValues("Name", configurationName, "Application", appName, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Configuration", configurationName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Bind Configuration")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.BindRequest{
		Names: []string{configurationName},
	}

	br, err := c.API.ConfigurationBindingCreate(request, c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

	if len(br.WasBound) > 0 {
		c.ui.Success().
			WithStringValue("Configuration", configurationName).
			WithStringValue("Application", appName).
			WithStringValue("Namespace", c.Settings.Namespace).
			Msg("Configuration Already Bound to Application.")

		return nil
	}

	c.ui.Success().
		WithStringValue("Configuration", configurationName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Configuration Bound to Application.")
	return nil
}

// UnbindConfiguration detaches the configuration specified by name from the named
// application, both in the targeted namespace.
func (c *EpinioClient) UnbindConfiguration(configurationName, appName string) error {
	log := c.Log.WithName("Unbind Configuration").
		WithValues("Name", configurationName, "Application", appName, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Configuration", configurationName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Unbind Configuration from Application")

	if err := c.TargetOk(); err != nil {
		return err
	}

	_, err := c.API.ConfigurationBindingDelete(c.Settings.Namespace, appName, configurationName)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Configuration", configurationName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Configuration Detached From Application.")
	return nil
}

// DeleteConfiguration deletes a configuration specified by name
func (c *EpinioClient) DeleteConfiguration(name string, unbind bool) error {
	log := c.Log.WithName("DeleteConfiguration").
		WithValues("Name", name, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Delete Configuration")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.ConfigurationDeleteRequest{
		Unbind: unbind,
	}

	var bound []string

	_, err := c.API.ConfigurationDelete(request, c.Settings.Namespace, name,
		func(response *http.Response, bodyBytes []byte, err error) error {
			// nothing special for internal errors and the like
			if response.StatusCode != http.StatusBadRequest {
				return err
			}

			// A bad request happens when
			//
			// 1. the configuration is still bound to one or more applications, and the
			//    response contains an array of their names.
			//
			// 2. the configuration is owned by a service and denied the request

			var apiError apierrors.ErrorResponse
			if err := json.Unmarshal(bodyBytes, &apiError); err != nil {
				return err
			}

			// [BELONG] keep in sync with same marker in the server
			if strings.Contains(apiError.Errors[0].Title, "Configuration belongs to service") {
				// (2.)
				return apiError.Errors[0]
			}

			// (1.)
			bound = strings.Split(apiError.Errors[0].Details, ",")
			return nil
		})
	if err != nil {
		return err
	}

	if len(bound) > 0 {
		sort.Strings(bound)
		sort.Strings(bound)
		msg := c.ui.Exclamation().WithTable("Bound Applications")

		for _, app := range bound {
			msg = msg.WithTableRow(app)
		}

		msg.Msg("Unable to delete configuration. It is still used by")
		c.ui.Exclamation().Compact().Msg("Use --unbind to force the issue")

		return nil
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Configuration Removed.")
	return nil
}

// UpdateConfiguration updates a configuration specified by name and information about removed keys and changed assignments.
// TODO: Allow underscores in configuration names (right now they fail because of kubernetes naming rules for secrets)
func (c *EpinioClient) UpdateConfiguration(name string, removedKeys []string, assignments map[string]string) error {
	log := c.Log.WithName("Update Configuration").
		WithValues("Name", name, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	msg := c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Settings.Namespace).
		WithTable("Parameter", "Op", "Value")

	for _, removed := range removedKeys {
		msg = msg.WithTableRow(removed, "remove", "")
	}

	changed := []string{}
	for key := range assignments {
		changed = append(changed, key)
	}
	sort.Strings(changed)

	for _, key := range changed {
		msg = msg.WithTableRow(key, "add/change", assignments[key])
	}
	msg.Msg("Update Configuration")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.ConfigurationUpdateRequest{
		Remove: removedKeys,
		Set:    assignments,
	}

	_, err := c.API.ConfigurationUpdate(request, c.Settings.Namespace, name)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Configuration Changes Saved.")

	return nil
}

// CreateConfiguration creates a configuration specified by name and key/value dictionary
// TODO: Allow underscores in configuration names (right now they fail because of kubernetes naming rules for secrets)
func (c *EpinioClient) CreateConfiguration(name string, dict []string) error {
	log := c.Log.WithName("Create Configuration").
		WithValues("Name", name, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	data := make(map[string]string)
	msg := c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Settings.Namespace).
		WithTable("Parameter", "Value", "Access Path")
	for i := 0; i < len(dict); i += 2 {
		key := dict[i]
		value := dict[i+1]
		path := fmt.Sprintf("/configurations/%s/%s", name, key)
		msg = msg.WithTableRow(key, value, path)
		data[key] = value
	}
	msg.Msg("Create Configuration")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.ConfigurationCreateRequest{
		Name: name,
		Data: data,
	}

	_, err := c.API.ConfigurationCreate(request, c.Settings.Namespace)
	if err != nil {
		return err
	}

	c.ui.Exclamation().
		Msg("Beware, the shown access paths are only available in the application's container")

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Configuration Saved.")
	return nil
}

// ConfigurationDetails shows the information of a configuration specified by name
func (c *EpinioClient) ConfigurationDetails(name string) error {
	log := c.Log.WithName("Configuration Details").
		WithValues("Name", name, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Configuration Details")

	if err := c.TargetOk(); err != nil {
		return err
	}

	resp, err := c.API.ConfigurationShow(c.Settings.Namespace, name)
	if err != nil {
		return err
	}
	configurationDetails := resp.Configuration.Details
	boundApps := resp.Configuration.BoundApps

	sort.Strings(boundApps)

	c.ui.Note().
		WithStringValue("Created", resp.Meta.CreatedAt.String()).
		WithStringValue("User", resp.Configuration.Username).
		WithStringValue("Type", resp.Configuration.Type).
		WithStringValue("Origin", resp.Configuration.Origin).
		WithStringValue("Used-By", strings.Join(boundApps, ", ")).
		Msg("")

	msg := c.ui.Success()

	if len(configurationDetails) > 0 {
		msg = msg.WithTable("Parameter", "Value", "Access Path")

		keys := make([]string, 0, len(configurationDetails))
		for k := range configurationDetails {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			msg = msg.WithTableRow(k, configurationDetails[k],
				fmt.Sprintf("/configurations/%s/%s", name, k))
		}

		msg.Msg("")
	} else {
		msg.Msg("No parameters")
	}

	c.ui.Exclamation().
		Msg("Beware, the shown access paths are only available in the application's container")
	return nil
}
