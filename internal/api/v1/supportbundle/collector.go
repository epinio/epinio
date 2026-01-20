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

package supportbundle

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/mholt/archives"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// Collector handles collection of logs from various Epinio components.
// It organizes logs by component type and writes them to files in the bundle directory.
type Collector struct {
	cluster   *kubernetes.Cluster
	bundleDir string
	tailLines *int64
	logger    *zap.SugaredLogger
}

// NewCollector creates a new log collector
func NewCollector(cluster *kubernetes.Cluster, bundleDir string, tailLines int64, logger *zap.SugaredLogger) *Collector {
	return &Collector{
		cluster:   cluster,
		bundleDir: bundleDir,
		tailLines: &tailLines,
		logger:    logger,
	}
}

// CollectEpinioServerLogs collects logs from Epinio server pods (current and previous containers)
func (c *Collector) CollectEpinioServerLogs(ctx context.Context) error {
	selector := labels.NewSelector()
	req, err := labels.NewRequirement("app.kubernetes.io/component", selection.Equals, []string{"epinio-server"})
	if err != nil {
		return errors.Wrap(err, "failed to create label requirement")
	}
	selector = selector.Add(*req)

	// Collect logs including previous containers
	return c.collectPodLogsWithPrevious(ctx, "epinio-server", SupportBundleNamespace, selector)
}

// CollectEpinioUILogs collects logs from Epinio UI pods (current and previous containers)
func (c *Collector) CollectEpinioUILogs(ctx context.Context) error {
	// Epinio UI is typically deployed with app.kubernetes.io/name=epinio-ui
	selector := labels.NewSelector()
	req, err := labels.NewRequirement("app.kubernetes.io/name", selection.Equals, []string{"epinio-ui"})
	if err != nil {
		return errors.Wrap(err, "failed to create label requirement")
	}
	selector = selector.Add(*req)

	// Try to collect from epinio namespace first, fallback to all namespaces
	// Collect logs including previous containers
	err = c.collectPodLogsWithPrevious(ctx, "epinio-ui", SupportBundleNamespace, selector)
	if err != nil {
		// If not found in epinio namespace, try all namespaces
		return c.collectPodLogsWithPrevious(ctx, "epinio-ui", "", selector)
	}
	return nil
}

// CollectStagingJobLogs collects logs from recent staging jobs (last 24 hours)
func (c *Collector) CollectStagingJobLogs(ctx context.Context) error {
	selector := labels.NewSelector()
	stagingReq, err := labels.NewRequirement("app.kubernetes.io/component", selection.Equals, []string{"staging"})
	if err != nil {
		return errors.Wrap(err, "failed to create label requirement")
	}
	selector = selector.Add(*stagingReq)

	// Get all staging jobs
	jobs, err := c.cluster.ListJobs(ctx, "", selector.String())
	if err != nil {
		return errors.Wrap(err, "failed to list staging jobs")
	}

	// Filter to recent jobs (within the time window)
	cutoffTime := time.Now().Add(-RecentStagingJobsWindow)
	var recentJobs []string
	for _, job := range jobs.Items {
		if job.CreationTimestamp.After(cutoffTime) {
			recentJobs = append(recentJobs, job.Name)
		}
	}

	if len(recentJobs) == 0 {
		c.logger.Infow("no recent staging jobs found")
		return nil
	}

	// Collect logs for each recent staging job by finding pods with job-name label
	for _, jobName := range recentJobs {
		jobSelector := labels.NewSelector()
		// Add staging component requirement explicitly
		jobStagingReq, err := labels.NewRequirement("app.kubernetes.io/component", selection.Equals, []string{"staging"})
		if err != nil {
			c.logger.Errorw("failed to create staging requirement for job", "error", err, "job", jobName)
			continue
		}
		jobSelector = jobSelector.Add(*jobStagingReq)
		
		// Add job-name requirement
		jobReq, err := labels.NewRequirement("job-name", selection.Equals, []string{jobName})
		if err != nil {
			c.logger.Errorw("failed to create job-name requirement", "error", err, "job", jobName)
			continue
		}
		jobSelector = jobSelector.Add(*jobReq)

		dirName := fmt.Sprintf("staging-jobs/%s", jobName)
		// For staging jobs, apply the 24-hour time window
		if err := c.collectPodLogs(ctx, dirName, "", jobSelector, true); err != nil {
			c.logger.Errorw("failed to collect logs for staging job", "error", err, "job", jobName)
			// Continue with other jobs
		}
	}

	return nil
}

// CollectMinioLogs collects logs from Minio pods
func (c *Collector) CollectMinioLogs(ctx context.Context) error {
	// Minio is typically deployed with app=minio or app.kubernetes.io/name=minio
	// Try multiple common label selectors
	selectors := []labels.Selector{}

	// Try app=minio
	sel1 := labels.NewSelector()
	req1, err := labels.NewRequirement("app", selection.Equals, []string{"minio"})
	if err == nil {
		sel1 = sel1.Add(*req1)
		selectors = append(selectors, sel1)
	}

	// Try app.kubernetes.io/name=minio
	sel2 := labels.NewSelector()
	req2, err := labels.NewRequirement("app.kubernetes.io/name", selection.Equals, []string{"minio"})
	if err == nil {
		sel2 = sel2.Add(*req2)
		selectors = append(selectors, sel2)
	}

	// Try to find Minio in common namespaces
	namespaces := []string{SupportBundleNamespace, "minio", "default"}
	for _, ns := range namespaces {
		for _, selector := range selectors {
			if err := c.collectPodLogs(ctx, "minio", ns, selector, false); err == nil {
				return nil // Successfully collected
			}
		}
	}

	// If not found in specific namespaces, try all namespaces
	for _, selector := range selectors {
		if err := c.collectPodLogs(ctx, "minio", "", selector, false); err == nil {
			return nil
		}
	}

	c.logger.Infow("Minio pods not found, skipping Minio logs")
	return nil
}

// CollectRegistryLogs collects logs from container registry pods
func (c *Collector) CollectRegistryLogs(ctx context.Context) error {
	// Container registry could be deployed with various labels
	// Common patterns: app=registry, app.kubernetes.io/name=registry, or part of a chart
	selectors := []labels.Selector{}

	// Try app=registry
	sel1 := labels.NewSelector()
	req1, err := labels.NewRequirement("app", selection.Equals, []string{"registry"})
	if err == nil {
		sel1 = sel1.Add(*req1)
		selectors = append(selectors, sel1)
	}

	// Try app.kubernetes.io/name=registry
	sel2 := labels.NewSelector()
	req2, err := labels.NewRequirement("app.kubernetes.io/name", selection.Equals, []string{"registry"})
	if err == nil {
		sel2 = sel2.Add(*req2)
		selectors = append(selectors, sel2)
	}

	// Try docker-registry (common Helm chart name)
	sel3 := labels.NewSelector()
	req3, err := labels.NewRequirement("app.kubernetes.io/name", selection.Equals, []string{"docker-registry"})
	if err == nil {
		sel3 = sel3.Add(*req3)
		selectors = append(selectors, sel3)
	}

	// Try to find registry in common namespaces
	namespaces := []string{SupportBundleNamespace, "registry", "default"}
	for _, ns := range namespaces {
		for _, selector := range selectors {
			if err := c.collectPodLogs(ctx, "registry", ns, selector, false); err == nil {
				return nil // Successfully collected
			}
		}
	}

	// If not found in specific namespaces, try all namespaces
	for _, selector := range selectors {
		if err := c.collectPodLogs(ctx, "registry", "", selector, false); err == nil {
			return nil
		}
	}

	c.logger.Infow("Container registry pods not found, skipping registry logs")
	return nil
}

// CollectApplicationLogs collects logs from all Epinio applications
func (c *Collector) CollectApplicationLogs(ctx context.Context) error {
	c.logger.Infow("starting application logs collection")
	
	// Build base selector with component requirement
	baseSelector := labels.NewSelector()
	req1, err := labels.NewRequirement("app.kubernetes.io/component", selection.Equals, []string{"application"})
	if err != nil {
		return errors.Wrap(err, "failed to create component label requirement")
	}
	baseSelector = baseSelector.Add(*req1)

	// First try with managed-by label for more specific matching
	selectorWithManagedBy := labels.NewSelector()
	selectorWithManagedBy = selectorWithManagedBy.Add(*req1)
	
	req2, err := labels.NewRequirement("app.kubernetes.io/managed-by", selection.Equals, []string{"epinio"})
	if err != nil {
		return errors.Wrap(err, "failed to create managed-by label requirement")
	}
	selectorWithManagedBy = selectorWithManagedBy.Add(*req2)

	c.logger.Debugw("checking for application pods", "selector_with_managed_by", selectorWithManagedBy.String(), "selector_base", baseSelector.String())

	// Check if any pods exist with managed-by label
	podList, err := c.cluster.Kubectl.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: selectorWithManagedBy.String(),
	})
	if err != nil {
		c.logger.Errorw("failed to check pods with managed-by label, trying without it", "error", err, "selector", selectorWithManagedBy.String())
		// Fall through to try without managed-by
	} else if len(podList.Items) > 0 {
		// Found pods with managed-by label, use that selector
		c.logger.Infow("found application pods with managed-by label", "count", len(podList.Items), "selector", selectorWithManagedBy.String())
		return c.collectApplicationLogsByNamespace(ctx, selectorWithManagedBy)
	}

	// Fallback: try without managed-by requirement
	// (Some Helm charts may not set managed-by on pods, only on other resources)
	c.logger.Debugw("no pods found with managed-by label, trying without it", "selector", selectorWithManagedBy.String(), "fallback_selector", baseSelector.String())
	return c.collectApplicationLogsByNamespace(ctx, baseSelector)
}

// collectApplicationLogsByNamespace collects application logs organized by namespace
func (c *Collector) collectApplicationLogsByNamespace(ctx context.Context, selector labels.Selector) error {
	// Get all application pods
	podList, err := c.cluster.Kubectl.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to list application pods")
	}

	c.logger.Infow("found application pods", "count", len(podList.Items), "selector", selector.String())

	// Log details about found pods for debugging
	if len(podList.Items) > 0 {
		for _, pod := range podList.Items {
			c.logger.Debugw("found application pod",
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"app_name", pod.Labels["app.kubernetes.io/name"],
				"component", pod.Labels["app.kubernetes.io/component"],
				"part_of", pod.Labels["app.kubernetes.io/part-of"],
				"managed_by", pod.Labels["app.kubernetes.io/managed-by"],
				"containers", len(pod.Spec.Containers),
				"app_container", pod.Annotations["epinio.io/app-container"])
		}
	}

	// Group pods by namespace and application name
	podsByApp := make(map[string]map[string][]corev1.Pod) // namespace -> app -> pods
	skippedPods := 0
	for _, pod := range podList.Items {
		ns := pod.Namespace
		appName := pod.Labels["app.kubernetes.io/name"]
		if appName == "" {
			c.logger.Debugw("skipping pod without app.kubernetes.io/name label",
				"pod", pod.Name, "namespace", ns, "labels", pod.Labels)
			skippedPods++
			continue
		}

		if podsByApp[ns] == nil {
			podsByApp[ns] = make(map[string][]corev1.Pod)
		}
		podsByApp[ns][appName] = append(podsByApp[ns][appName], pod)
	}

	if skippedPods > 0 {
		c.logger.Infow("skipped pods without app name label", "count", skippedPods)
	}

	if len(podsByApp) == 0 {
		c.logger.Infow("no application pods found matching selector - application logs will not be included in bundle", 
			"selector", selector.String(), 
			"pods_found", len(podList.Items),
			"pods_skipped", skippedPods)
		return nil
	}

	// Collect logs for each application
	totalApps := 0
	totalPods := 0
	for ns, apps := range podsByApp {
		for appName, pods := range apps {
			totalApps++
			totalPods += len(pods)
			c.logger.Infow("collecting logs for application",
				"namespace", ns, "app", appName, "pod_count", len(pods))
			dirName := fmt.Sprintf("applications/%s/%s", ns, appName)
			for _, pod := range pods {
				// Application logs: no time window, collect all available logs
				if err := c.collectPodLogsDirect(ctx, dirName, pod, false); err != nil {
					c.logger.Errorw("failed to collect logs for application pod",
						"error", err, "namespace", ns, "app", appName, "pod", pod.Name)
					// Continue with other pods
				} else {
					c.logger.Debugw("successfully collected logs for application pod",
						"namespace", ns, "app", appName, "pod", pod.Name)
				}
			}
		}
	}

	c.logger.Infow("completed application log collection", 
		"total_apps", totalApps, "total_pods", totalPods, "namespaces", len(podsByApp))
	
	if totalApps == 0 && totalPods == 0 {
		c.logger.Infow("WARNING: no application logs were collected - check if application pods exist with label app.kubernetes.io/component=application")
	}

	return nil
}

// collectPodLogs collects logs from pods matching the selector
// applyTimeWindow: if true, only collect logs from last 24 hours (for staging jobs)
func (c *Collector) collectPodLogs(ctx context.Context, dirName, namespace string, selector labels.Selector, applyTimeWindow bool) error {
	podList, err := c.cluster.Kubectl.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to list pods")
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods found matching selector: %s", selector.String())
	}

	for _, pod := range podList.Items {
		if err := c.collectPodLogsDirect(ctx, dirName, pod, applyTimeWindow); err != nil {
			c.logger.Errorw("failed to collect logs for pod", "error", err, "pod", pod.Name, "namespace", pod.Namespace)
			// Continue with other pods
		}
	}

	return nil
}

// collectPodLogsWithPrevious collects logs from pods including previous containers
func (c *Collector) collectPodLogsWithPrevious(ctx context.Context, dirName, namespace string, selector labels.Selector) error {
	podList, err := c.cluster.Kubectl.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return errors.Wrap(err, "failed to list pods")
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods found matching selector: %s", selector.String())
	}

	for _, pod := range podList.Items {
		if err := c.collectPodLogsDirectWithPrevious(ctx, dirName, pod); err != nil {
			c.logger.Errorw("failed to collect logs for pod", "error", err, "pod", pod.Name, "namespace", pod.Namespace)
			// Continue with other pods
		}
	}

	return nil
}

// collectPodLogsDirect collects logs from a specific pod
// applyTimeWindow: if true, only collect logs from last 24 hours (for staging jobs)
func (c *Collector) collectPodLogsDirect(ctx context.Context, dirName string, pod corev1.Pod, applyTimeWindow bool) error {
	// Create directory for this component
	componentDir := filepath.Join(c.bundleDir, dirName)
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create component directory")
	}

	// Get the app container name from annotation if available (for application pods)
	appContainerName := pod.Annotations["epinio.io/app-container"]

	// Collect logs from all containers in the pod (including sidecars)
	containersCollected := 0
	for _, container := range pod.Spec.Containers {
		containerName := container.Name

		logFileName := fmt.Sprintf("%s-%s-%s.log", pod.Namespace, pod.Name, containerName)
		logFilePath := filepath.Join(componentDir, logFileName)

		c.logger.Debugw("collecting logs for container", 
			"pod", pod.Name, "container", containerName, "namespace", pod.Namespace,
			"is_app_container", containerName == appContainerName)

		if err := c.writePodLogs(ctx, pod.Namespace, pod.Name, containerName, logFilePath, applyTimeWindow); err != nil {
			c.logger.Errorw("failed to write logs for container",
				"error", err, "pod", pod.Name, "container", containerName)
			// Continue with other containers
		} else {
			containersCollected++
		}
	}

	if containersCollected == 0 {
		containerNames := make([]string, len(pod.Spec.Containers))
		for i, c := range pod.Spec.Containers {
			containerNames[i] = c.Name
		}
		c.logger.Infow("WARNING: no containers collected from pod",
			"pod", pod.Name, "namespace", pod.Namespace, "total_containers", len(pod.Spec.Containers),
			"container_names", containerNames)
	}

	// Also collect from init containers if they exist (typically for staging jobs)
	for _, container := range pod.Spec.InitContainers {
		containerName := container.Name
		logFileName := fmt.Sprintf("%s-%s-%s-init.log", pod.Namespace, pod.Name, containerName)
		logFilePath := filepath.Join(componentDir, logFileName)

		if err := c.writePodLogs(ctx, pod.Namespace, pod.Name, containerName, logFilePath, applyTimeWindow); err != nil {
			c.logger.Errorw("failed to write logs for init container",
				"error", err, "pod", pod.Name, "container", containerName)
		}
	}

	return nil
}

// collectPodLogsDirectWithPrevious collects logs from a specific pod including previous containers
func (c *Collector) collectPodLogsDirectWithPrevious(ctx context.Context, dirName string, pod corev1.Pod) error {
	// Create directory for this component
	componentDir := filepath.Join(c.bundleDir, dirName)
	if err := os.MkdirAll(componentDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create component directory")
	}

	// Collect logs from all containers in the pod (current and previous)
	for _, container := range pod.Spec.Containers {
		containerName := container.Name
		
		// Current container logs
		logFileName := fmt.Sprintf("%s-%s-%s.log", pod.Namespace, pod.Name, containerName)
		logFilePath := filepath.Join(componentDir, logFileName)
		if err := c.writePodLogs(ctx, pod.Namespace, pod.Name, containerName, logFilePath, false); err != nil {
			c.logger.Errorw("failed to write logs for container",
				"error", err, "pod", pod.Name, "container", containerName)
			// Continue with other containers
		}

		// Previous container logs
		prevLogFileName := fmt.Sprintf("%s-%s-%s-previous.log", pod.Namespace, pod.Name, containerName)
		prevLogFilePath := filepath.Join(componentDir, prevLogFileName)
		if err := c.writePreviousPodLogs(ctx, pod.Namespace, pod.Name, containerName, prevLogFilePath); err != nil {
			c.logger.Errorw("failed to write previous container logs",
				"error", err, "pod", pod.Name, "container", containerName)
			// Continue - previous logs may not exist
		}
	}

	// Also collect from init containers if they exist (usually no previous for init containers)
	for _, container := range pod.Spec.InitContainers {
		containerName := container.Name
		logFileName := fmt.Sprintf("%s-%s-%s-init.log", pod.Namespace, pod.Name, containerName)
		logFilePath := filepath.Join(componentDir, logFileName)

		if err := c.writePodLogs(ctx, pod.Namespace, pod.Name, containerName, logFilePath, false); err != nil {
			c.logger.Errorw("failed to write logs for init container",
				"error", err, "pod", pod.Name, "container", containerName)
		}
	}

	return nil
}

// writePodLogs writes pod logs to a file using Kubernetes API directly
// applyTimeWindow: if true, only collect logs from last 24 hours (for staging jobs)
func (c *Collector) writePodLogs(ctx context.Context, namespace, podName, containerName, filePath string, applyTimeWindow bool) error {
	// Create log file
	file, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}
	defer func() {
		if err := file.Close(); err != nil {
			c.logger.Errorw("failed to close log file", "error", err, "file", filePath)
		}
	}()

	// Get pod logs using Kubernetes API
	podLogOptions := &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  c.tailLines,
		Timestamps: true,
		Previous:   false, // current container
	}

	// Only apply time window for staging jobs (to limit log volume)
	if applyTimeWindow {
		sinceSeconds := int64(RecentStagingJobsWindow.Seconds())
		podLogOptions.SinceSeconds = &sinceSeconds
		c.logger.Debugw("applying time window filter", "since_seconds", sinceSeconds, "pod", podName, "container", containerName)
	}

	req := c.cluster.Kubectl.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		// If container doesn't exist or pod is not ready, log at Info level so it's visible
		// This helps debug why logs aren't being collected
		c.logger.Infow("could not get logs for container - logs will not be included in bundle", 
			"pod", podName, "container", containerName, "namespace", namespace, "error", err.Error())
		// Write a note to the file instead of leaving it empty
		errorMsg := fmt.Sprintf("Logs not available for container %s in pod %s (namespace: %s): %s\n", containerName, podName, namespace, err.Error())
		_, writeErr := file.WriteString(errorMsg)
		if writeErr != nil {
			return errors.Wrap(writeErr, "failed to write error message to file")
		}
		// Return nil to continue with other containers, but the error is logged at Info level
		return nil
	}
	defer func() {
		if err := stream.Close(); err != nil {
			c.logger.Errorw("failed to close log stream", "error", err, "pod", podName, "container", containerName)
		}
	}()

	// Copy logs to file
	bytesWritten, err := io.Copy(file, stream)
	if err != nil {
		return errors.Wrap(err, "failed to copy logs to file")
	}

	c.logger.Debugw("successfully wrote pod logs to file", 
		"pod", podName, "container", containerName, "namespace", namespace, 
		"file", filePath, "bytes", bytesWritten)

	return nil
}

// writePreviousPodLogs writes previous container logs to a file
func (c *Collector) writePreviousPodLogs(ctx context.Context, namespace, podName, containerName, filePath string) error {
	// Create log file
	file, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}
	defer func() {
		if err := file.Close(); err != nil {
			c.logger.Errorw("failed to close log file", "error", err, "file", filePath)
		}
	}()

	// Get previous container logs using Kubernetes API
	podLogOptions := &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  c.tailLines,
		Timestamps: true,
		Previous:   true, // previous container
	}

	req := c.cluster.Kubectl.CoreV1().Pods(namespace).GetLogs(podName, podLogOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		// Previous container logs may not exist (e.g., first run, container hasn't restarted)
		// This is expected behavior, not an error
		c.logger.Debugw("previous container logs not available", "pod", podName, "container", containerName, "namespace", namespace, "note", "this is normal if container hasn't restarted")
		// Write a note to the file explaining why logs aren't available
		note := fmt.Sprintf("Previous container logs not available for container %s in pod %s (namespace: %s).\nThis is normal for first run or if the container hasn't restarted.\n", containerName, podName, namespace)
		_, writeErr := file.WriteString(note)
		if writeErr != nil {
			return errors.Wrap(writeErr, "failed to write message to file")
		}
		return nil
	}
	defer func() {
		if err := stream.Close(); err != nil {
			c.logger.Errorw("failed to close log stream", "error", err, "pod", podName, "container", containerName)
		}
	}()

	// Copy logs to file
	_, err = io.Copy(file, stream)
	if err != nil {
		return errors.Wrap(err, "failed to copy logs to file")
	}

	return nil
}

// CreateArchive creates a tar.gz archive of all collected log files.
// It walks the bundle directory, collects all log files, and creates a compressed tar archive.
// Returns the path to the created archive file.
func (c *Collector) CreateArchive(ctx context.Context) (string, error) {
	archivePath := filepath.Join(c.bundleDir, "support-bundle.tar.gz")

	// Create a map of files to include in the archive
	files := make(map[string]string)

	// Walk the bundle directory and collect all log files
	err := filepath.Walk(c.bundleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "error accessing path %s", path)
		}

		// Skip directories and the archive file itself (if it exists)
		if info.IsDir() || path == archivePath {
			return nil
		}

		// Get relative path from bundle directory for archive structure
		relPath, err := filepath.Rel(c.bundleDir, path)
		if err != nil {
			return errors.Wrapf(err, "failed to get relative path for %s", path)
		}

		files[path] = relPath
		return nil
	})

	if err != nil {
		return "", errors.Wrap(err, "failed to walk bundle directory")
	}

	if len(files) == 0 {
		return "", errors.New("no files found to include in archive")
	}

	c.logger.Debugw("creating archive", "file_count", len(files), "archive_path", archivePath)

	// Use the archives library to create the tar.gz
	filesFromDisk, err := archives.FilesFromDisk(ctx, nil, files)
	if err != nil {
		return "", errors.Wrap(err, "failed to create files from disk")
	}

	// Create the archive file
	outFile, err := os.Create(archivePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to create archive file")
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			c.logger.Errorw("failed to close archive file", "error", err, "path", archivePath)
		}
	}()

	// Create compressed tar archive
	format := archives.CompressedArchive{
		Archival: archives.Tar{},
	}

	if err := format.Archive(ctx, outFile, filesFromDisk); err != nil {
		return "", errors.Wrap(err, "failed to create archive")
	}

	// Verify archive was created successfully
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to verify archive was created")
	}

	c.logger.Infow("archive created successfully", "path", archivePath, "size_bytes", archiveInfo.Size(), "file_count", len(files))

	return archivePath, nil
}

