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

	"github.com/pkg/errors"
)

// // CreateGitconfig creates a gitconfig
// func (c *EpinioClient) CreateGitconfig(gitconfig string) error {
// 	log := c.Log.WithName("CreateGitconfig").WithValues("Gitconfig", gitconfig)
// 	log.Info("start")
// 	defer log.Info("return")

// 	c.ui.Note().
// 		WithStringValue("Name", gitconfig).
// 		Msg("Creating gitconfig...")

// 	errorMsgs := validation.IsDNS1123Subdomain(gitconfig)
// 	if len(errorMsgs) > 0 {
// 		return fmt.Errorf("Gitconfig's name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
// 	}

// 	_, err := c.API.GitconfigCreate(models.GitconfigCreateRequest{Name: gitconfig})
// 	if err != nil {
// 		return err
// 	}

// 	c.ui.Success().Msg("Gitconfig created.")

// 	return nil
// }

// GitconfigsMatching returns all Epinio gi tconfigurations having the specified prefix in their name
func (c *EpinioClient) GitconfigsMatching(prefix string) []string {
	log := c.Log.WithName("GitconfigsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	resp, err := c.API.GitconfigsMatch(prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	log.Info("matches", "found", result)
	return result
}

func (c *EpinioClient) Gitconfigs() error {
	log := c.Log.WithName("Gitconfigs")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Listing Git Configurations")

	details.Info("list gitconfigs")

	gitconfigs, err := c.API.Gitconfigs()
	if err != nil {
		return err
	}

	sort.Sort(gitconfigs)
	msg := c.ui.Success().WithTable("ID", "Provider", "URL", "User/Org", "Repository", "Skip SSL", "Username")

	for _, gitconfig := range gitconfigs {
		msg = msg.WithTableRow(
			gitconfig.Meta.Name,
			// gitconfig.Meta.CreatedAt.String(),
			string(gitconfig.Provider),
			gitconfig.URL,
			gitconfig.UserOrg,
			gitconfig.Repository,
			fmt.Sprintf("%v", gitconfig.SkipSSL),
			gitconfig.Username,
		)
	}

	msg.Msg("Epinio Git Configurations:")

	return nil
}

// DeleteGitconfig deletes a Gitconfig
func (c *EpinioClient) DeleteGitconfig(gitconfigs []string, all bool) error {

	if all && len(gitconfigs) > 0 {
		return errors.New("Conflict between --all and given git configurations")
	}
	if !all && len(gitconfigs) == 0 {
		return errors.New("No git configurations specified for deletion")
	}

	if all {
		c.ui.Note().
			Msg("Querying Git Configurations for Deletion...")

		if err := c.TargetOk(); err != nil {
			return err
		}

		// Using the match API with a query matching everything. Avoids transmission
		// of full configuration data and having to filter client-side.
		match, err := c.API.GitconfigsMatch("")
		if err != nil {
			return err
		}
		if len(match.Names) == 0 {
			c.ui.Exclamation().Msg("No gitconfigs found to delete")
			return nil
		}

		gitconfigs = match.Names
		sort.Strings(gitconfigs)
	}

	namesCSV := strings.Join(gitconfigs, ", ")
	log := c.Log.WithName("DeleteGitconfig").WithValues("Gitconfigs", namesCSV)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Git Configurations", namesCSV).
		Msg("Deleting git configurations...")

	s := c.ui.Progressf("Deleting %s", gitconfigs)
	defer s.Stop()

	go c.trackDeletion(gitconfigs, func() []string {
		match, err := c.API.GitconfigsMatch("")
		if err != nil {
			return []string{}
		}
		return match.Names
	})

	_, err := c.API.GitconfigDelete(gitconfigs)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Git Configurations deleted.")

	return nil
}

// ShowGitconfig shows a single Git configuration
func (c *EpinioClient) ShowGitconfig(gcName string) error {
	log := c.Log.WithName("ShowGitconfig").WithValues("Gitconfig", gcName)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", gcName).
		Msg("Showing gitconfig...")

	gitconfig, err := c.API.GitconfigShow(gcName)
	if err != nil {
		return err
	}

	c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Name", gitconfig.Meta.Name).
		// WithTableRow("Created", gitconfig.Meta.CreatedAt.String()).
		WithTableRow("Provider", string(gitconfig.Provider)).
		WithTableRow("URL", gitconfig.URL).
		WithTableRow("User/Org", gitconfig.UserOrg).
		WithTableRow("Repository", gitconfig.Repository).
		WithTableRow("Skip SSL", fmt.Sprintf("%v", gitconfig.SkipSSL)).
		WithTableRow("Username", gitconfig.Username).
		Msg("Details:")

	return nil
}
