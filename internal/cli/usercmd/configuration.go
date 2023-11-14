// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usercmd

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/util/validation"
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

	if c.ui.JSONEnabled() {
		return c.ui.JSON(configurations)
	}

	details.Info("show configurations")

	msg = c.ui.Success()
	if all {
		msg = msg.WithTable("Namespace", "Name", "Created", "Type", "Origin", "Applications")

		for _, configuration := range configurations {
			apps := strings.Join(configuration.Configuration.BoundApps, ", ")

			msg = msg.WithTableRow(
				configuration.Meta.Namespace,
				configuration.Meta.Name,
				configuration.Meta.CreatedAt.String(),
				configuration.Configuration.Type,
				configuration.Configuration.Origin,
				apps)
		}
	} else {
		msg = msg.WithTable("Name", "Created", "Type", "Origin", "Applications")

		for _, configuration := range configurations {
			apps := strings.Join(configuration.Configuration.BoundApps, ", ")

			msg = msg.WithTableRow(
				configuration.Meta.Name,
				configuration.Meta.CreatedAt.String(),
				configuration.Configuration.Type,
				configuration.Configuration.Origin,
				apps)
		}
	}

	msg.Msg("Epinio Configurations:")

	return nil
}

// ConfigurationMatching returns all Epinio configurations having the specified prefix
// in their name.
func (c *EpinioClient) ConfigurationMatching(prefix string) []string {
	log := c.Log.WithName("ConfigurationMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	c.API.DisableVersionWarning()

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

// DeleteConfiguration deletes one or more configurations, specified by name
func (c *EpinioClient) DeleteConfiguration(names []string, unbind, all bool) error {
	if all {
		c.ui.Note().
			WithStringValue("Namespace", c.Settings.Namespace).
			Msg("Querying Configurations for Deletion...")

		if err := c.TargetOk(); err != nil {
			return err
		}

		// Using the match API with a query matching everything. Avoids transmission
		// of full configuration data and having to filter client-side.
		match, err := c.API.ConfigurationMatch(c.Settings.Namespace, "")
		if err != nil {
			return err
		}
		if len(match.Names) == 0 {
			c.ui.Exclamation().Msg("No configurations found to delete")
			return nil
		}

		names = match.Names
		sort.Strings(names)
	}

	namesCSV := strings.Join(names, ", ")
	log := c.Log.WithName("DeleteConfiguration").
		WithValues("Names", namesCSV, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Names", namesCSV).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Deleting Configurations...")

	if !all {
		if err := c.TargetOk(); err != nil {
			return err
		}
	}

	request := models.ConfigurationDeleteRequest{
		Unbind: unbind,
	}

	var bound []string

	s := c.ui.Progressf("Deleting %s in %s", names, c.Settings.Namespace)
	defer s.Stop()

	go c.trackDeletion(names, func() []string {
		match, err := c.API.ConfigurationMatch(c.Settings.Namespace, "")
		if err != nil {
			return []string{}
		}
		return match.Names
	})

	_, err := c.API.ConfigurationDelete(request, c.Settings.Namespace, names)
	if err != nil {
		epinioAPIError := &client.APIError{}
		// something bad happened
		if !errors.As(err, &epinioAPIError) {
			return err
		}

		// the API error is something different from a bad request (500?). Do not handle.
		if epinioAPIError.StatusCode != http.StatusBadRequest {
			return err
		}

		// A bad request happens when
		//
		// 1. the configuration is still bound to one or more applications, and the
		//    response contains an array of their names.
		//
		// 2. the configuration is owned by a service and denied the request

		firstErr := epinioAPIError.Err.Errors[0]

		// [BELONG] keep in sync with same markers in the server
		if strings.Contains(firstErr.Title, "belongs to service") {
			// (2.)
			return firstErr
		}

		// (1.)
		bound = strings.Split(firstErr.Details, ",")
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
		WithStringValue("Names", namesCSV).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Configurations Removed.")
	return nil
}

// UpdateConfiguration updates a configuration specified by name and information about removed keys and changed assignments.
// TODO: Allow underscores in configuration names (right now they fail because of kubernetes naming rules for secrets)
func (c *EpinioClient) UpdateConfiguration(name string, removedKeys []string, assignments map[string]string) error {
	log := c.Log.WithName("Update Configuration").
		WithValues("Name", name, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.showChanges("Configuration", name, removedKeys, assignments)

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
		msg = msg.WithTableRow(key, transformForDisplay(value), path)
		data[key] = value
	}
	msg.Msg("Create Configuration")

	if err := c.TargetOk(); err != nil {
		return err
	}

	errorMsgs := validation.IsDNS1123Subdomain(name)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("Configuration's name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
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

	if c.ui.JSONEnabled() {
		return c.ui.JSON(resp)
	}

	configurationDetails := resp.Configuration.Details
	boundApps := resp.Configuration.BoundApps
	siblings := resp.Configuration.Siblings

	sort.Strings(boundApps)
	sort.Strings(siblings)

	c.ui.Note().
		WithStringValue("Created", resp.Meta.CreatedAt.String()).
		WithStringValue("User", resp.Configuration.Username).
		WithStringValue("Type", resp.Configuration.Type).
		WithStringValue("Origin", resp.Configuration.Origin).
		WithStringValue("Used-By", strings.Join(boundApps, ", ")).
		WithStringValue("Siblings", strings.Join(siblings, ", ")).
		Msg("")

	msg := c.ui.Success()

	if len(configurationDetails) > 0 {
		msg = msg.WithTable("Parameter", "Value", "Access Path")

		path := name
		if resp.Configuration.Origin != "" {
			path = resp.Configuration.Origin

			// Use the configuration's siblings (and itself) to determine if
			// disambiguation is needed and if yes, where in the total order this
			// configuration falls. See [CS-DISAMBI].
			if len(siblings) > 0 {
				siblings = append(siblings, name)
				sort.Strings(siblings)
				serial := 0
				for idx, cName := range siblings {
					serial = idx
					if cName == name {
						break
					}
				}
				if serial > 1 {
					path = fmt.Sprintf("%s-%d", path, serial)
				}
			}
		}

		keys := make([]string, 0, len(configurationDetails))
		for k := range configurationDetails {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			value := transformForDisplay(configurationDetails[k])
			msg = msg.WithTableRow(k, value, fmt.Sprintf("/configurations/%s/%s", path, k))
		}

		msg.Msg("")

		c.ui.Exclamation().
			Msg("Beware, the shown access paths are only available in the application's container")
	} else {
		msg.Msg("No parameters")
	}

	return nil
}

func transformForDisplay(v string) string {
	// Consider: Count and truncate by runes, not bytes.
	// - https://pkg.go.dev/unicode/utf8@go1.20.5#RuneCountInString
	// - https://go.dev/blog/strings (Libraries, foreach/range)
	limit := 70

	// Pass short strings as-is
	if len(v) <= limit {
		return v
	}

	// and truncate long strings
	return fmt.Sprintf("%s (hiding %d additional bytes)", v[:limit], len(v)-limit)
}

func (c *EpinioClient) showChanges(label, name string, removedKeys []string, assignments map[string]string) {
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
	msg.Msg("Update " + label)
}
