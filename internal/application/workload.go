package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubectl/pkg/util/podutils"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

type AppConfigurationBind struct {
	configuration string // name of the configuration getting bound
	resource      string // name of the kube secret to mount as volume to make the configuration params available in the app
}

type AppConfigurationBindList []AppConfigurationBind

// Workload manages applications that are deployed. It provides workload
// (deployments) specific actions for the application model.
type Workload struct {
	app     models.AppRef
	cluster *kubernetes.Cluster
	name    string
}

// NewWorkload constructs and returns a workload representation from an application reference.
func NewWorkload(cluster *kubernetes.Cluster, app models.AppRef) *Workload {
	return &Workload{cluster: cluster, app: app}
}

func ToBinds(ctx context.Context, configurations configurations.ConfigurationList, appName string, userName string) (AppConfigurationBindList, error) {
	bindings := AppConfigurationBindList{}

	for _, configuration := range configurations {
		bindResource, err := configuration.GetSecret(ctx)
		if err != nil {
			return AppConfigurationBindList{}, err
		}
		bindings = append(bindings, AppConfigurationBind{
			resource:      bindResource.Name,
			configuration: configuration.Name,
		})
	}

	return bindings, nil
}

func (b AppConfigurationBindList) ToVolumesArray() []corev1.Volume {
	volumes := []corev1.Volume{}

	for _, binding := range b {
		volumes = append(volumes, corev1.Volume{
			Name: binding.configuration,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: binding.resource,
				},
			},
		})
	}

	return volumes
}

func (b AppConfigurationBindList) ToMountsArray() []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{}

	for _, binding := range b {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      binding.configuration,
			ReadOnly:  true,
			MountPath: fmt.Sprintf("/configurations/%s", binding.configuration),
		})
	}

	return mounts
}

func (b AppConfigurationBindList) ToNames() []string {
	names := []string{}

	for _, binding := range b {
		names = append(names, binding.configuration)
	}

	return names
}

// Pods is a helper, it returns the Pods belonging to the Deployment of the workload.
func (a *Workload) Pods(ctx context.Context) ([]corev1.Pod, error) {
	podList, err := a.cluster.Kubectl.CoreV1().Pods(a.app.Namespace).List(
		ctx, metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{
				"app.kubernetes.io/component": "application",
				"app.kubernetes.io/name":      a.app.Name,
				"app.kubernetes.io/part-of":   a.app.Namespace,
			}).String(),
		})
	if err != nil {
		return []corev1.Pod{}, err
	}

	return podList.Items, nil
}

func (a *Workload) PodNames(ctx context.Context) ([]string, error) {
	podList, err := a.Pods(ctx)
	if err != nil {
		return []string{}, err
	}

	result := []string{}
	for _, p := range podList {
		result = append(result, p.Name)
	}

	return result, nil
}

// Replicas returns a slice of models.PodInfo. Each PodInfo matches a Pod belonging to
// the application Deployment (workload).
func (a *Workload) Replicas(ctx context.Context) (map[string]*models.PodInfo, error) {
	result := map[string]*models.PodInfo{}

	pods, err := a.Pods(ctx)
	if err != nil {
		return result, err
	}
	podMetrics, err := a.getPodMetrics(ctx)
	if err != nil {
		return result, err
	}

	result = a.generatePodInfo(pods)

	if err = a.populatePodMetrics(result, podMetrics); err != nil {
		return result, err
	}

	return result, nil
}

// Get returns the state of the app deployment encoded in the workload.
func (a *Workload) Get(ctx context.Context, desiredReplicas int32) (*models.AppDeployment, error) {

	// Information about the active workload.
	// The data is pulled out of the list of Pods associated with the application.
	// The originally asked the app's Deployment resource. With app charts the existence
	// of such a resource cannot be guarantueed any longer.

	podList, err := a.Pods(ctx)
	if err != nil {
		return nil, err
	}

	// No pods => no workload
	if len(podList) == 0 {
		return nil, nil
	}

	var readyReplicas int32
	var createdAt time.Time
	var stageID string
	var username string
	var controllerName string

	if len(podList) > 0 {
		// Pods found. replace defaults with actual information.

		// Initialize various pieces from the first pod ...
		createdAt = podList[0].ObjectMeta.CreationTimestamp.Time
		stageID = podList[0].ObjectMeta.Labels["epinio.io/stage-id"]
		username = podList[0].ObjectMeta.Labels["app.kubernetes.io/created-by"]
		controllerName = podList[0].ObjectMeta.Labels["epinio.io/app-container"]

		for _, pod := range podList {
			// Choose oldest time of all pods.
			if createdAt.After(pod.ObjectMeta.CreationTimestamp.Time) {
				createdAt = pod.ObjectMeta.CreationTimestamp.Time
			}

			// Count ready pods - A temp is used to avoid `Implicit memory aliasing in for loop`.
			tmp := pod
			if podutils.IsPodReady(&tmp) {
				readyReplicas = readyReplicas + 1
			}
		}
	}

	a.name = controllerName

	status := fmt.Sprintf("%d/%d", readyReplicas, desiredReplicas)

	routes, err := ListRoutes(ctx, a.cluster, a.app)
	if err != nil {
		routes = []string{err.Error()}
	}

	replicas, err := a.Replicas(ctx)
	if err != nil {
		status = pkgerrors.Wrap(err, "failed to get replica details").Error()
	}

	return &models.AppDeployment{
		Name:            controllerName,
		Active:          true,
		CreatedAt:       createdAt.Format(time.RFC3339), // ISO 8601
		Replicas:        replicas,
		Username:        username,
		StageID:         stageID,
		Status:          status,
		Routes:          routes,
		DesiredReplicas: desiredReplicas,
		ReadyReplicas:   readyReplicas,
	}, nil
}

func (a *Workload) getPodMetrics(ctx context.Context) ([]metricsv1beta1.PodMetrics, error) {
	result := []metricsv1beta1.PodMetrics{}

	selector := fmt.Sprintf(`app.kubernetes.io/name=%s`, a.app.Name)

	metricsClient, err := metrics.NewForConfig(a.cluster.RestConfig)
	if err != nil {
		return result, err
	}

	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(a.app.Namespace).
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return result, err
	}

	return podMetrics.Items, nil
}

func (a *Workload) generatePodInfo(pods []corev1.Pod) map[string]*models.PodInfo {
	result := map[string]*models.PodInfo{}

	for i, pod := range pods {
		restarts := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == a.name {
				restarts += cs.RestartCount
			}
		}

		result[pod.Name] = &models.PodInfo{
			Name:      pod.Name,
			Restarts:  restarts,
			Ready:     podutils.IsPodReady(&pods[i]),
			CreatedAt: pod.ObjectMeta.CreationTimestamp.Time.Format(time.RFC3339), // ISO 8601
		}
	}

	return result
}

func (a *Workload) populatePodMetrics(podInfos map[string]*models.PodInfo, podMetrics []metricsv1beta1.PodMetrics) error {
	for _, podMetric := range podMetrics {
		if _, podExists := podInfos[podMetric.Name]; !podExists {
			continue // should not happen but just making sure metrics match pods
		}

		cpuUsage := resource.NewQuantity(0, resource.DecimalSI)
		memUsage := resource.NewQuantity(0, resource.BinarySI)

		podContainers := podMetric.Containers
		for _, container := range podContainers {
			cpuUsage.Add(*container.Usage.Cpu())
			memUsage.Add(*container.Usage.Memory())
		}

		// cpu * 1000 -> milliCPUs (rounded)
		milliCPUs := int64(math.Round(cpuUsage.ToDec().AsApproximateFloat64() * 1000))

		mem, ok := memUsage.AsInt64()
		if !ok {
			return pkgerrors.Errorf("couldn't get memory usage as an integer, memUsage.AsDec = %T %+v\n", memUsage.AsDec(), memUsage.AsDec())
		}

		podInfos[podMetric.Name].MemoryBytes = mem
		podInfos[podMetric.Name].MilliCPUs = milliCPUs
	}

	return nil
}
