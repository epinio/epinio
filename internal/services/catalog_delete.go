package services

import (
	"context"

	"github.com/epinio/epinio/internal/helmchart"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteCatalogService removes the named Service CR from the epinio
// namespace.
func (s *ServiceClient) DeleteCatalogService(
	ctx context.Context,
	name string,
) error {
	deleteError := s.serviceKubeClient.
		Namespace(helmchart.Namespace()).
		Delete(ctx, name, metav1.DeleteOptions{})

	if deleteError != nil {
		return errors.Wrap(deleteError, "deleting catalog service")
	}

	return nil
}
