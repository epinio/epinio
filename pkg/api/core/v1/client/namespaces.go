package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/avast/retry-go"

	"github.com/epinio/epinio/helpers"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// NamespaceCreate creates a namespace
func (c *Client) NamespaceCreate(req models.NamespaceCreateRequest) (models.Response, error) {
	var resp models.Response

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	details := c.log.V(1) // NOTE: Increment of level, not absolute.

	var data []byte
	err = retry.Do(
		func() error {
			details.Info("create org", "org", req.Name)
			data, err = c.post(api.Routes.Path("Namespaces"), string(b))
			return err
		},
		retry.RetryIf(func(err error) bool {
			if r, ok := err.(interface{ StatusCode() int }); ok {
				return helpers.RetryableCode(r.StatusCode())
			}
			retry := helpers.Retryable(err.Error())

			details.Info("create error", "error", err.Error(), "retry", retry)
			return retry
		}),
		retry.OnRetry(func(n uint, err error) {
			details.WithValues(
				"tries", fmt.Sprintf("%d/%d", n, duration.RetryMax),
				"error", err.Error(),
			).Info("Retrying to create namespace")
		}),
		retry.Delay(time.Second),
		retry.Attempts(duration.RetryMax),
	)
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// NamespaceDelete deletes a namespace
func (c *Client) NamespaceDelete(org string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("NamespaceDelete", org))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// NamespacesMatch returns all matching namespaces for the prefix
func (c *Client) NamespacesMatch(prefix string) (models.NamespacesMatchResponse, error) {
	resp := models.NamespacesMatchResponse{}

	data, err := c.get(api.Routes.Path("NamespacesMatch", prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// Namespaces returns a list of namespaces
func (c *Client) Namespaces() (models.NamespaceList, error) {
	resp := models.NamespaceList{}

	data, err := c.get(api.Routes.Path("Namespaces"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}
