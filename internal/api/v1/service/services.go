package service

import (
	"context"

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

/**

TODO:
We need a CRD to keep tracks of the released instances.
We could have one more CRD (i.e.: ServiceRelease) or we switch to the Rancher helm-controller
and use their HelmChart CRD to keep track of them.
In Any case after the installation is done we want to label the Opaque secrets that came out the deploy
in order to show them as Configurations, and be able to bind them to the services they belong to.

TODO2:
A service may output more than one secrets, so `epinio service bind` could bind all of them at once.

TODO3:
Show the type of the configuration and the service it belongs to (if any) in the `epinio configuration list/show`

TODO4:
remove the hardcoded service/release name during the `epinio service create`

*/
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

	err = kubeServiceClient.CreateRelease(ctx, namespace, service.Name, createRequest.ReleaseName)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = helmDeployService(ctx, cluster.RestConfig, *service, namespace, createRequest.ReleaseName)
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
	releaseName string,
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

	go func() {
		_, err = client.InstallOrUpgradeChart(context.Background(), &hc.ChartSpec{
			ReleaseName: names.ReleaseName(releaseName),
			ChartName:   service.HelmChart,
			Namespace:   namespace,
			Atomic:      true,
			Timeout:     duration.ToDeployment(),
			// TODO handle values
			// 		ValuesYaml: fmt.Sprintf(`
			// commonLabels:
			//   "%s": "true"
			// `, configurations.ConfigurationLabelKey),
			ReuseValues: true,
		})
	}()

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
