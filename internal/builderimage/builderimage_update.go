package builderimage

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Update applies a partial update to an existing BuilderImage CR. Empty fields
// in the request are left untouched.
func Update(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
	req models.BuilderImageUpdateRequest,
) error {
	client, err := cluster.ClientBuilderImage()
	if err != nil {
		return err
	}

	existing, err := client.
		Namespace(helmchart.Namespace()).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	spec, ok := existing.Object["spec"].(map[string]interface{})
	if !ok {
		spec = map[string]interface{}{}
		existing.Object["spec"] = spec
	}

	if req.Image != "" {
		spec["image"] = req.Image
	}
	if req.Description != "" {
		spec["description"] = req.Description
	}
	if req.ShortDescription != "" {
		spec["shortDescription"] = req.ShortDescription
	}

	_, err = client.
		Namespace(helmchart.Namespace()).
		Update(ctx, existing, metav1.UpdateOptions{})

	return err
}
