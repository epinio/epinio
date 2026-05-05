package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	var appChartRequestSpec models.AppChartRequestSpec
	bindError := c.BindJSON(&appChartRequestSpec)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	var appChartRequest = &models.AppChartRequest{
		Spec: appChartRequestSpec,
	}

	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(appChartRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	chart := &unstructured.Unstructured{Object: content}

	chart.SetKind("AppChart")
	chart.SetAPIVersion("application.epinio.io/v1")

	// Set the metadata name (Required for kubectl to find it)
	// Assuming your spec or a header provides a name; otherwise, hardcode for testing:
	chart.SetName("my-custom-chart")
	chart.SetNamespace("epinio")

	// Set the labels so Epinio and kubectl can filter/find it
	chart.SetLabels(map[string]string{
		"app.kubernetes.io/managed-by": "epinio",
		"epinio.io/area":               "epinio",
	})

	_, appChartError := appchart.Create(ctx, cluster, chart)

	if appChartError != nil {
		return nil
	}

	response.OK(c)
	return nil
}
