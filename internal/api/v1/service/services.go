package service

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/services"
	"github.com/gin-gonic/gin"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/rest"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	hc "github.com/mittwald/go-helm-client"
)

func (ctr Controller) Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	var createRequest models.ServiceCreateRequest
	err := c.BindJSON(&createRequest)
	if err != nil {
		return apierror.BadRequest(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	service, err := kubeServiceClient.Get(ctx, createRequest.Name)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = helmDeployService(ctx, cluster.RestConfig, *service, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}

func helmDeployService(
	ctx context.Context,
	restConfig *rest.Config,
	service models.Service,
	namespace string,
) error {

	client, err := getHelmClient(restConfig, namespace)
	if err != nil {
		return err
	}

	err = client.AddOrUpdateChartRepo(repo.Entry{
		Name: service.HelmRepo.Name,
		URL:  service.HelmRepo.URL,
	})
	if err != nil {
		return err
	}

	release, err := client.InstallOrUpgradeChart(ctx, &hc.ChartSpec{
		ReleaseName: names.ReleaseName(service.Name),
		ChartName:   service.HelmChart,
		Namespace:   namespace,
		Wait:        true,
		Atomic:      true,
		Timeout:     duration.ToDeployment(),
		// TODO handle values
		ValuesYaml: `
commonLabels:
  "epinio.io/mylabel": "foobar"
`,
		ReuseValues: true,
	})
	fmt.Printf("%+v\n", release)

	return err
}

func getHelmClient(restConfig *rest.Config, namespace string) (hc.Client, error) {
	return hc.NewClientFromRestConf(&hc.RestConfClientOptions{
		RestConfig: restConfig,
		Options: &hc.Options{
			Namespace:        namespace,
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Linting:          true,
			Debug:            true,
		},
	})
}
