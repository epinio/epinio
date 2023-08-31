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
	"os"
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"
)

// CreateGitconfig creates a gitconfig
func (c *EpinioClient) CreateGitconfig(id,
	providerString, url, user, password, userorg, repo, certfile string,
	skipssl bool) error {

	log := c.Log.WithName("CreateGitconfig").WithValues("gitconfig", id)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", id).
		WithStringValue("Provider", providerString).
		WithStringValue("Url", url).
		WithStringValue("User/Org", userorg).
		WithStringValue("Repository", repo).
		WithStringValue("Username", user).
		WithStringValue("Password", password).
		WithBoolValue("Skip SSL", skipssl).
		WithStringValue("Certificates from", certfile).
		Msg("Creating gitconfig...")

	errorMsgs := validation.IsDNS1123Subdomain(id)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("The git configuration's id must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
	}

	provider, err := models.GitProviderFromString(providerString)
	if err != nil {
		return err
	}

	certs := []byte{}

	if certfile != "" {
		content, err := os.ReadFile(certfile)
		if err != nil {
			return errors.Wrapf(err, "filesystem error")
		}
		certs = content
	}

	_, err = c.API.GitconfigCreate(models.GitconfigCreateRequest{
		ID:           id,
		Provider:     provider,
		URL:          url,
		UserOrg:      userorg,
		Repository:   repo,
		Username:     user,
		Password:     password,
		SkipSSL:      skipssl,
		Certificates: certs,
	})
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Git configuration created.")

	return nil
}

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

	c.ui.Note().Msg("Listing git configurations")

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

	msg.Msg("Git configurations:")

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
			c.ui.Exclamation().Msg("No git configurations found to delete")
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
		WithStringValue("Git configurations", namesCSV).
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

	c.ui.Success().Msg("Git configurations deleted.")

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
