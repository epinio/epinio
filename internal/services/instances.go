package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/panjf2000/ants"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Get returns a Service "instance" object if one is exist, or nil otherwise.
// Also returns an error if one occurs.
func (s *ServiceClient) Get(ctx context.Context, namespace, name string) (*models.Service, error) {
	var service models.Service

	helmChartName := models.ServiceHelmChartName(name, namespace)
	srv, err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).Get(ctx, helmChartName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "fetching the service instance")
	}

	catalogServiceName := ""
	for k, v := range srv.GetLabels() {
		if k == ServiceLabelKey {
			catalogServiceName = v
			break
		}
	}

	// Helmchart is not labeled, act as if service is "not found"
	if catalogServiceName == "" {
		return nil, nil
	}

	var catalogServicePrefix string
	_, err = s.GetCatalogService(ctx, catalogServiceName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			catalogServicePrefix = "[Missing] "
		} else {
			return nil, err
		}
	}

	targetNamespace, found, err := unstructured.NestedString(srv.UnstructuredContent(), "spec", "targetNamespace")
	if err != nil {
		return nil, errors.Wrapf(err, "looking up targetNamespace as a string")
	}
	if !found {
		return nil, errors.New("targetNamespace field not found")
	}

	service = models.Service{
		Name:           name,
		Namespace:      targetNamespace,
		CatalogService: fmt.Sprintf("%s%s", catalogServicePrefix, catalogServiceName),
	}

	logger := tracelog.NewLogger().WithName("ServiceStatus")
	serviceStatus, err := helm.Status(ctx, logger, s.kubeClient, targetNamespace, helmChartName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			serviceStatus = "Not Ready" // The installation job is still running?
		} else {
			return &service, errors.Wrap(err, "finding helm release status")
		}
	}

	service.Status = serviceStatus

	return &service, nil
}

func (s *ServiceClient) Create(ctx context.Context, namespace, name string, catalogService models.CatalogService) error {
	helmChart := &helmapiv1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "helm.cattle.io/v1",
			Kind:       "HelmChart",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      models.ServiceHelmChartName(name, namespace),
			Namespace: helmchart.Namespace(),
			Labels: map[string]string{
				ServiceLabelKey: catalogService.Name,
			},
		},
		Spec: helmapiv1.HelmChartSpec{
			TargetNamespace: namespace,
			Chart:           catalogService.HelmChart,
			Repo:            catalogService.HelmRepo.URL,
			ValuesContent:   catalogService.Values,
		},
	}

	mapHelmChart, err := runtime.DefaultUnstructuredConverter.ToUnstructured(helmChart)
	if err != nil {
		return errors.Wrap(err, "error converting helmChart to unstructured")
	}

	unstructureHelmChart := &unstructured.Unstructured{}
	unstructureHelmChart.SetUnstructuredContent(mapHelmChart)

	_, err = s.helmChartsKubeClient.Namespace(helmchart.Namespace()).Create(
		ctx, unstructureHelmChart, metav1.CreateOptions{})
	return errors.Wrap(err, "error creating helm chart")
}

// DeleteAll deletes all helmcharts installed on the specified namespace.
// It's used to cleanup before a namespace is deleted.
// The targetNamespace is not the namespace where the helmchart resource resides
// (that would be `epinio`) but the `targetNamespace` field of the helmchart.
// Since FieldSelector on CRDs can't be used with arbitrary fields, we need
// to delete them one by one (otherwise we could use `DeleteCollection`).
func (s *ServiceClient) DeleteAll(ctx context.Context, targetNamespace string) error {
	list, err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).List(ctx,
		metav1.ListOptions{
			LabelSelector: ServiceLabelKey, // Existence of the label key
		},
	)
	if err != nil {
		return errors.Wrap(err, "error listing helm charts")
	}

	const maxConcurrent = 100
	errChan := make(chan error)
	var wg, errWg sync.WaitGroup
	var loopErr error

	errWg.Add(1)
	go func() {
		for err := range errChan {
			loopErr = err
			break
		}
		errWg.Done()
	}()

	p, err := ants.NewPoolWithFunc(maxConcurrent, func(i interface{}) {
		err := s.helmChartsKubeClient.Namespace(helmchart.Namespace()).Delete(
			ctx, i.(string), metav1.DeleteOptions{})
		if err != nil {
			errChan <- err
		}
		wg.Done()
	}, ants.WithExpiryDuration(30*time.Second))
	if err != nil {
		return err
	}

	for _, item := range list.Items {
		n, found, err := unstructured.NestedString(item.UnstructuredContent(), "spec", "targetNamespace")
		if err != nil {
			errChan <- err
		}

		if found && n == targetNamespace {
			wg.Add(1)
			err = p.Invoke(item.GetName())
			if err != nil {
				errChan <- err
			}
		}
	}
	defer p.Release()

	wg.Wait()
	close(errChan)
	errWg.Wait()

	return loopErr
}
