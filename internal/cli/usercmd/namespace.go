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
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/fatih/color"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"
)

// CreateNamespace creates a namespace
func (c *EpinioClient) CreateNamespace(namespace string) error {
	log := c.Log.WithName("CreateNamespace").WithValues("Namespace", namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", namespace).
		Msg("Creating namespace...")

	errorMsgs := validation.IsDNS1123Subdomain(namespace)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("Namespace's name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
	}

	_, err := c.API.NamespaceCreate(models.NamespaceCreateRequest{Name: namespace})
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Namespace created.")

	return nil
}

// NamespacesMatching returns all Epinio namespaces having the specified prefix in their name
func (c *EpinioClient) NamespacesMatching(prefix string) []string {
	log := c.Log.WithName("NamespaceMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	c.API.DisableVersionWarning()

	resp, err := c.API.NamespacesMatch(prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	log.Info("matches", "found", result)
	return result
}

func (c *EpinioClient) Namespaces() error {
	log := c.Log.WithName("Namespaces")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Listing namespaces")

	details.Info("list namespaces")

	namespaces, err := c.API.Namespaces()
	if err != nil {
		return err
	}

	sort.Sort(namespaces)

	if c.ui.JSONEnabled() {
		return c.ui.JSON(namespaces)
	}

	msg := c.ui.Success().WithTable("Name", "Created", "Applications", "Configurations")

	for _, namespace := range namespaces {
		sort.Strings(namespace.Apps)
		sort.Strings(namespace.Configurations)
		msg = msg.WithTableRow(
			namespace.Meta.Name,
			namespace.Meta.CreatedAt.String(),
			strings.Join(namespace.Apps, ", "),
			strings.Join(namespace.Configurations, ", "))
	}

	msg.Msg("Epinio Namespaces:")

	return nil
}

// Target targets a namespace
func (c *EpinioClient) Target(namespace string) error {
	log := c.Log.WithName("Target").WithValues("Namespace", namespace)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	if namespace == "" {
		details.Info("query settings")
		c.ui.Success().
			WithStringValue("Currently targeted namespace", c.Settings.Namespace).
			Msg("")
		return nil
	}

	c.ui.Note().
		WithStringValue("Name", namespace).
		Msg("Targeting namespace...")

	// we don't need anything, just checking if the namespace exist and we have permissions
	_, err := c.API.NamespaceShow(namespace)
	if err != nil {
		return errors.Wrap(err, "error targeting namespace")
	}

	details.Info("set settings")
	c.Settings.Namespace = namespace
	err = c.Settings.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save settings")
	}

	c.ui.Success().Msg("Namespace targeted.")

	return nil
}

func (c *EpinioClient) TargetOk() error {
	if c.Settings.Namespace == "" {
		return errors.New("Internal Error: No namespace targeted")
	}
	return nil
}

// DeleteNamespace deletes a Namespace
func (c *EpinioClient) DeleteNamespace(namespaces []string, force, all bool) error {

	if all && len(namespaces) > 0 {
		return errors.New("Conflict between --all and given namespaces")
	}
	if !all && len(namespaces) == 0 {
		return errors.New("No namespaces specified for deletion")
	}

	if all {
		c.ui.Note().
			WithStringValue("Namespace", c.Settings.Namespace).
			Msg("Querying Namespaces for Deletion...")

		if err := c.TargetOk(); err != nil {
			return err
		}

		// Using the match API with a query matching everything. Avoids transmission
		// of full configuration data and having to filter client-side.
		match, err := c.API.NamespacesMatch("")
		if err != nil {
			return err
		}
		if len(match.Names) == 0 {
			c.ui.Exclamation().Msg("No namespaces found to delete")
			return nil
		}

		namespaces = match.Names
		sort.Strings(namespaces)
	}

	if !force {
		var m string
		if len(namespaces) == 1 {
			m = fmt.Sprintf("You are about to delete the namespace '%s' and everything it includes, i.e. applications, configurations, etc.",
				namespaces[0])
		} else {
			names := strings.Join(namespaces, ", ")
			m = fmt.Sprintf("You are about to delete %d namespaces (%s) and everything they include, i.e. applications, configurations, etc.",
				len(namespaces), names)
		}

		if !c.askConfirmation(m) {
			return errors.New("Cancelled by user")
		}
	}

	namesCSV := strings.Join(namespaces, ", ")
	log := c.Log.WithName("DeleteNamespace").WithValues("Namespaces", namesCSV)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespaces", namesCSV).
		Msg("Deleting namespaces...")

	s := c.ui.Progressf("Deleting %s", namespaces)
	defer s.Stop()

	go c.trackDeletion(namespaces, func() []string {
		match, err := c.API.NamespacesMatch("")
		if err != nil {
			return []string{}
		}
		return match.Names
	})

	_, err := c.API.NamespaceDelete(namespaces)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Namespaces deleted.")

	return nil
}

// ShowNamespace shows a Namespace
func (c *EpinioClient) ShowNamespace(namespace string) error {
	log := c.Log.WithName("ShowNamespace").WithValues("Namespace", namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", namespace).
		Msg("Showing namespace...")

	space, err := c.API.NamespaceShow(namespace)
	if err != nil {
		return err
	}

	if c.ui.JSONEnabled() {
		return c.ui.JSON(space)
	}

	msg := c.ui.Success().WithTable("Key", "Value")

	sort.Strings(space.Apps)
	sort.Strings(space.Configurations)

	msg = msg.
		WithTableRow("Name", space.Meta.Name).
		WithTableRow("Created", space.Meta.CreatedAt.String()).
		WithTableRow("Applications", strings.Join(space.Apps, "\n")).
		WithTableRow("Configurations", strings.Join(space.Configurations, "\n"))

	msg.Msg("Details:")

	return nil
}

// askConfirmation is a helper for CmdNamespaceDelete to confirm a deletion request
func (c *EpinioClient) askConfirmation(m string) bool {
	c.ui.Note().Msg(m)
	for {
		var s string
		c.ui.Question().WithAskString("", &s).Msg("Are you sure? (y/N): ")
		s = strings.ToLower(s)

		if s == "n" || s == "no" || s == "" {
			return false
		}
		if s == "y" || s == "yes" {
			return true
		}

		// Bad input, repeat question
		c.ui.Raw(color.RedString("Expected one of `y`, `yes`, `n`, or `no`"))
	}
}
