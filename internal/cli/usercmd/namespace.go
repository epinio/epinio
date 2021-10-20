package usercmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"
)

// CreateOrg creates an Org namespace
func (c *EpinioClient) CreateOrg(org string) error {
	log := c.Log.WithName("CreateNamespace").WithValues("Namespace", org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Creating namespace...")

	errorMsgs := validation.IsDNS1123Subdomain(org)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("%s: %s", "org name incorrect", strings.Join(errorMsgs, "\n"))
	}

	_, err := c.API.NamespaceCreate(models.NamespaceCreateRequest{Name: org})
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Namespace created.")

	return nil
}

// OrgsMatching returns all Epinio orgs having the specified prefix in their name
func (c *EpinioClient) OrgsMatching(prefix string) []string {
	log := c.Log.WithName("NamespaceMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	resp, err := c.API.NamespacesMatch(prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	log.Info("matches", "found", result)
	return result
}

func (c *EpinioClient) Orgs() error {
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
	msg := c.ui.Success().WithTable("Name", "Applications", "Services")

	for _, namespace := range namespaces {
		sort.Strings(namespace.Apps)
		sort.Strings(namespace.Services)
		msg = msg.WithTableRow(
			namespace.Name,
			strings.Join(namespace.Apps, ", "),
			strings.Join(namespace.Services, ", "))
	}

	msg.Msg("Epinio Namespaces:")

	return nil
}

// Target targets an org
func (c *EpinioClient) Target(org string) error {
	log := c.Log.WithName("Target").WithValues("Namespace", org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	if org == "" {
		details.Info("query config")
		c.ui.Success().
			WithStringValue("Currently targeted namespace", c.Config.Org).
			Msg("")
		return nil
	}

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Targeting namespace...")

	// TODO: Validation of the org name removed. Proper validation
	// of the targeted org is done by all the other commands using
	// it anyway. If we really want it here and now, implement an
	// `namespace show` command and API, and then use that API for the
	// validation.

	details.Info("set config")
	c.Config.Org = org
	err := c.Config.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save configuration")
	}

	c.ui.Success().Msg("Namespace targeted.")

	return nil
}

func (c *EpinioClient) TargetOk() error {
	if c.Config.Org == "" {
		return errors.New("Internal Error: No namespace targeted")
	}
	return nil
}

// DeleteOrg deletes an Org
func (c *EpinioClient) DeleteOrg(org string) error {
	log := c.Log.WithName("DeleteNamespace").WithValues("Namespace", org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Deleting namespace...")

	_, err := c.API.NamespaceDelete(org)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Namespace deleted.")

	return nil
}

// ShowOrg shows an Org
func (c *EpinioClient) ShowOrg(org string) error {
	log := c.Log.WithName("ShowNamespace").WithValues("Namespace", org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Showing namespace...")

	space, err := c.API.NamespaceShow(org)
	if err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Key", "Value")

	sort.Strings(space.Apps)
	sort.Strings(space.Services)

	msg = msg.WithTableRow("Name", space.Name).
		WithTableRow("Applications", strings.Join(space.Apps, "\n")).
		WithTableRow("Services", strings.Join(space.Services, "\n"))

	msg.Msg("Details:")

	return nil
}
