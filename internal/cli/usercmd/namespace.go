package usercmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"

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
		return fmt.Errorf("%s: %s", "namespace name incorrect", strings.Join(errorMsgs, "\n"))
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

// Target targets a namespace
func (c *EpinioClient) Target(namespace string) error {
	log := c.Log.WithName("Target").WithValues("Namespace", namespace)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	if namespace == "" {
		details.Info("query config")
		c.ui.Success().
			WithStringValue("Currently targeted namespace", c.Config.Namespace).
			Msg("")
		return nil
	}

	c.ui.Note().
		WithStringValue("Name", namespace).
		Msg("Targeting namespace...")

	details.Info("set config")
	c.Config.Namespace = namespace
	err := c.Config.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save configuration")
	}

	c.ui.Success().Msg("Namespace targeted.")

	return nil
}

func (c *EpinioClient) TargetOk() error {
	if c.Config.Namespace == "" {
		return errors.New("Internal Error: No namespace targeted")
	}
	return nil
}

// DeleteNamespace deletes a Namespace
func (c *EpinioClient) DeleteNamespace(namespace string) error {
	log := c.Log.WithName("DeleteNamespace").WithValues("Namespace", namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", namespace).
		Msg("Deleting namespace...")

	_, err := c.API.NamespaceDelete(namespace)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Namespace deleted.")

	return nil
}

// ShowNamepsace shows a Namespace
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

	msg := c.ui.Success().WithTable("Key", "Value")

	sort.Strings(space.Apps)
	sort.Strings(space.Services)

	msg = msg.WithTableRow("Name", space.Name).
		WithTableRow("Applications", strings.Join(space.Apps, "\n")).
		WithTableRow("Services", strings.Join(space.Services, "\n"))

	msg.Msg("Details:")

	return nil
}
