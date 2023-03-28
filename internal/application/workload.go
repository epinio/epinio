// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
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
	app             models.AppRef
	cluster         *kubernetes.Cluster
	name            string
	desiredReplicas int32
}

// NewWorkload constructs and returns a workload representation from an application reference.
func NewWorkload(cluster *kubernetes.Cluster, app models.AppRef, desiredReplicas int32) *Workload {
	return &Workload{cluster: cluster, app: app, desiredReplicas: desiredReplicas}
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

// AddApplicationPods is a helper for List. It loads all the epinio controlled pods in the
// namespace into memory, indexes them by namespace and application, and returns the resulting map
// of pod lists.
// ATTENTION: Using an empty string for the namespace loads the information from all namespaces.
func AddApplicationPods(auxiliary map[ConfigurationKey]AppData, ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[ConfigurationKey]AppData, error) {
	podList, err := cluster.Kubectl.CoreV1().Pods(namespace).List(
		ctx, metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{
				"app.kubernetes.io/component": "application",
			}).String(),
		})
	if err != nil {
		return nil, err
	}

	for _, pod := range podList.Items {
		appName := pod.Labels["app.kubernetes.io/name"]
		appNamespace := pod.Labels["app.kubernetes.io/part-of"]
		key := EncodeConfigurationKey(appName, appNamespace)

		if _, found := auxiliary[key]; !found {
			auxiliary[key] = AppData{}
		}

		data := auxiliary[key]

		data.pods = append(data.pods, pod)

		auxiliary[key] = data
	}

	return auxiliary, nil
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

// Replicas returns a map of models.PodInfo. Each PodInfo matches a Pod belonging to
// the application Deployment (workload).
func (a *Workload) replicas(pods []corev1.Pod, podMetrics []metricsv1beta1.PodMetrics) (map[string]*models.PodInfo, error) {
	result := a.generatePodInfo(pods)

	if err := a.populatePodMetrics(result, podMetrics); err != nil {
		return result, err
	}

	return result, nil
}

// Get returns the state of the app deployment encoded in the workload.
func (a *Workload) Get(ctx context.Context) (*models.AppDeployment, error) {

	// Information about the active workload.  The data is pulled out of the list of Pods
	// associated with the application.  It originally directly asked the app's Deployment
	// resource. With app charts the existence of such a resource cannot be guarantueed any
	// longer.

	podList, err := a.Pods(ctx)
	if err != nil {
		return nil, err
	}

	routes, err := ListRoutes(ctx, a.cluster, a.app)
	if err != nil {
		routes = []string{err.Error()}
	}

	// -- errors retrieving the pod metrics are ignored.
	// -- this will be reported later as `not available`.
	// note: The pod metrics are not nil in that cases, just an empty slice.
	// that is good, as that allows AFP below to still generate the basic pod info.
	podMetrics, err := a.getPodMetrics(ctx)
	if err != nil {
		// While the error is ignored, as the server can operate without metrics, and while
		// the missing metrics will be noted in the data shown to the user, it is logged so
		// that the operator can see this as well.
		requestctx.Logger(ctx).Error(err, "metrics not available")
	}

	return a.AssembleFromParts(ctx, podList, podMetrics, routes)
}

// AssembleFromParts is the core of Get constructing the deployment structure from the pods and
// auxiliary information explicitly given to it.
func (a *Workload) AssembleFromParts(
	ctx context.Context,
	podList []corev1.Pod,
	podMetrics []metricsv1beta1.PodMetrics,
	routes []string,
) (*models.AppDeployment, error) {
	// No pods => no workload
	if len(podList) == 0 {
		return nil, nil
	}

	var (
		readyReplicas  int32
		createdAt      time.Time
		stageID        string
		username       string
		controllerName string
	)

	if len(podList) > 0 {
		// Pods found. replace defaults with actual information.

		// Initialize various pieces from the first pod ...
		createdAt = podList[0].ObjectMeta.CreationTimestamp.Time
		stageID = podList[0].ObjectMeta.Labels["epinio.io/stage-id"]
		controllerName = podList[0].ObjectMeta.Labels["epinio.io/app-container"]
		username = podList[0].ObjectMeta.Annotations[models.EpinioCreatedByAnnotation]

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

	// Order is important. Required before replicas is called.
	a.name = controllerName

	var status string
	var replicas map[string]*models.PodInfo
	var err error
	if podMetrics != nil {
		replicas, err = a.replicas(podList, podMetrics)
		if err != nil {
			status = pkgerrors.Wrap(err, "failed to get replica details").Error()
		}
	}

	if status == "" {
		status = fmt.Sprintf("%d/%d", readyReplicas, a.desiredReplicas)
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
		DesiredReplicas: a.desiredReplicas,
		ReadyReplicas:   readyReplicas,
	}, nil
}

// GetPodMetrics is a helper for List. It loads all the pot metrics for epinio controlled pods in
// the namespace into memory, indexes them by pod name, and returns the resulting map of metrics
// lists. The user, List, selects the metrics it needs for an application based on the application's
// pods.
// ATTENTION: Using an empty string for the namespace loads the information from all namespaces.
func GetPodMetrics(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[string]metricsv1beta1.PodMetrics, error) {
	result := make(map[string]metricsv1beta1.PodMetrics)

	metricsClient, err := metrics.NewForConfig(cluster.RestConfig)
	if err != nil {
		return nil, err
	}

	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=epinio",
		})
	if err != nil {
		return nil, err
	}

	for _, metric := range podMetrics.Items {
		result[metric.ObjectMeta.Name] = metric
	}

	return result, nil
}

// getPodMetrics loads the pod metrics for the specific application into memory and returns the
// resulting slice.
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

		podInfos[podMetric.Name].MetricsOk = true
		podInfos[podMetric.Name].MemoryBytes = mem
		podInfos[podMetric.Name].MilliCPUs = milliCPUs
	}

	return nil
}
