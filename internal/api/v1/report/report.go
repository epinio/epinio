// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package report

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/version"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	reportTimeLayout = "Mon Jan _2 15:04:05 MST 2006"
)

// Nodes returns a rancher-like node report in JSON by default.
// To receive a text report, use ?format=text.
func Nodes(c *gin.Context) errors.APIErrors {
	ctx := c.Request.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return errors.InternalError(err)
	}

	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return errors.InternalError(err)
	}

	platform := cluster.GetPlatform().String()

	nodes, err := cluster.Kubectl.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.InternalError(err)
	}

	systemPods, err := cluster.Kubectl.CoreV1().Pods(helmchart.Namespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.InternalError(err)
	}

	now := time.Now().UTC()
	appScaleReports := buildAppScaleReports(ctx, cluster)
	report := buildReport(now, platform, kubeVersion, systemPods.Items, nodes.Items, appScaleReports)

	if strings.EqualFold(c.Query("format"), "text") {
		body := renderTextReport(report)
		c.Data(200, "text/plain; charset=utf-8", []byte(body))
		return nil
	}

	response.OKReturn(c, report)
	return nil
}

func buildReport(now time.Time, platform, kubeVersion string, pods []corev1.Pod, nodes []corev1.Node, apps []models.AppScaleReport) models.ReportResponse {
	podReports := buildSystemPodReports(now, pods)
	nodeReports := buildNodeReports(nodes)

	clusterCreatedAt := earliestNodeCreation(nodes)
	cluster := models.ClusterReport{
		ID:          "local",
		Name:        "local",
		KubeVersion: kubeVersion,
		Provider:    platform,
		CreatedAt:   clusterCreatedAt,
		Nodes:       nodeReports,
		NodeCount:   len(nodeReports),
	}

	return models.ReportResponse{
		GeneratedAt:       now.Format(time.RFC3339),
		GeneratedAtHuman:  now.Format(reportTimeLayout),
		EpinioVersion:     version.Version,
		KubernetesVersion: kubeVersion,
		Platform:          platform,
		SystemPods:        podReports,
		Clusters:          []models.ClusterReport{cluster},
		Applications:      apps,
		TotalNodeCount:    len(nodeReports),
	}
}

func buildAppScaleReports(ctx context.Context, cluster *kubernetes.Cluster) []models.AppScaleReport {
	appRefs, err := application.ListAppRefs(ctx, cluster, "")
	if err != nil {
		helpers.Logger.Infow("failed to list apps for report", "error", err)
		return nil
	}

	reports := make([]models.AppScaleReport, 0, len(appRefs))
	for _, appRef := range appRefs {
		appCR, err := application.Get(ctx, cluster, appRef)
		if err != nil {
			helpers.Logger.Infow("failed to load app for report", "error", err, "app", appRef.Name, "namespace", appRef.Namespace)
			continue
		}

		desired, err := application.Scaling(ctx, cluster, appRef)
		if err != nil {
			helpers.Logger.Infow("failed to read app scaling for report", "error", err, "app", appRef.Name, "namespace", appRef.Namespace)
			continue
		}

		scaleEvent := findLatestScaleEvent(ctx, cluster, appRef)
		report := models.AppScaleReport{
			Name:             appRef.Name,
			Namespace:        appRef.Namespace,
			CreatedAt:        appCR.GetCreationTimestamp().Format(time.RFC3339),
			DesiredInstances: desired,
		}

		if scaleEvent != nil {
			report.LastScaleAt = scaleEvent.timestamp.Format(time.RFC3339)
			report.LastScaleBy = scaleEvent.username
			report.LastScaleFrom = scaleEvent.from
			report.LastScaleTo = scaleEvent.to
		}

		reports = append(reports, report)
	}

	sort.Slice(reports, func(i, j int) bool {
		if reports[i].Namespace == reports[j].Namespace {
			return reports[i].Name < reports[j].Name
		}
		return reports[i].Namespace < reports[j].Namespace
	})

	return reports
}

func buildSystemPodReports(now time.Time, pods []corev1.Pod) []models.SystemPodReport {
	reports := make([]models.SystemPodReport, 0, len(pods))
	for _, pod := range pods {
		ready, total := podReadyCount(pod)
		reports = append(reports, models.SystemPodReport{
			Name:     pod.Name,
			Ready:    fmt.Sprintf("%d/%d", ready, total),
			Status:   string(pod.Status.Phase),
			Restarts: podRestartCount(pod),
			Age:      formatAge(now, pod.CreationTimestamp.Time),
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Name < reports[j].Name
	})

	return reports
}

func podReadyCount(pod corev1.Pod) (int, int) {
	total := len(pod.Status.ContainerStatuses)
	if total == 0 {
		return 0, 0
	}

	ready := 0
	for _, status := range pod.Status.ContainerStatuses {
		if status.Ready {
			ready++
		}
	}

	return ready, total
}

func podRestartCount(pod corev1.Pod) int32 {
	restarts := int32(0)
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
	}
	return restarts
}

func buildNodeReports(nodes []corev1.Node) []models.NodeReport {
	reports := make([]models.NodeReport, 0, len(nodes))
	for _, node := range nodes {
		addresses := nodeAddresses(node)
		etcd, controlPlane, worker := nodeRoles(node)
		reports = append(reports, models.NodeReport{
			ID:                      node.Name,
			Address:                 strings.Join(addresses, ","),
			Etcd:                    etcd,
			ControlPlane:            controlPlane,
			Worker:                  worker,
			CPU:                     node.Status.Capacity.Cpu().String(),
			RAM:                     node.Status.Capacity.Memory().String(),
			OS:                      node.Status.NodeInfo.OSImage,
			ContainerRuntimeVersion: node.Status.NodeInfo.ContainerRuntimeVersion,
			CreatedAt:               node.CreationTimestamp.Format(time.RFC3339),
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].ID < reports[j].ID
	})

	return reports
}

func nodeAddresses(node corev1.Node) []string {
	addresses := make([]string, 0, len(node.Status.Addresses))
	for _, address := range node.Status.Addresses {
		if address.Address == "" {
			continue
		}
		addresses = append(addresses, address.Address)
	}
	return addresses
}

func nodeRoles(node corev1.Node) (bool, bool, bool) {
	labels := node.Labels
	etcd := labelExists(labels, "node-role.kubernetes.io/etcd")
	controlPlane := labelExists(labels, "node-role.kubernetes.io/control-plane") ||
		labelExists(labels, "node-role.kubernetes.io/master")
	worker := labelExists(labels, "node-role.kubernetes.io/worker")

	if !etcd && !controlPlane && !worker {
		worker = true
	}

	return etcd, controlPlane, worker
}

func labelExists(labels map[string]string, key string) bool {
	if labels == nil {
		return false
	}
	_, found := labels[key]
	return found
}

func earliestNodeCreation(nodes []corev1.Node) string {
	var earliest time.Time
	for _, node := range nodes {
		if earliest.IsZero() || node.CreationTimestamp.Time.Before(earliest) {
			earliest = node.CreationTimestamp.Time
		}
	}

	if earliest.IsZero() {
		return ""
	}

	return earliest.Format(time.RFC3339)
}

func formatAge(now, created time.Time) string {
	if created.IsZero() {
		return "unknown"
	}

	age := now.Sub(created)
	if age < 0 {
		age = 0
	}

	days := int(age.Hours() / 24)
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}

	hours := int(age.Hours())
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}

	minutes := int(age.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	seconds := int(age.Seconds())
	return fmt.Sprintf("%ds", seconds)
}

func renderTextReport(report models.ReportResponse) string {
	var b strings.Builder

	fmt.Fprintln(&b, "Epinio Systems Summary Report")
	fmt.Fprintln(&b, "============================")
	fmt.Fprintf(&b, "Run on %s\n\n", report.GeneratedAtHuman)

	if len(report.SystemPods) > 0 {
		fmt.Fprintln(&b, "NAME                               READY   STATUS    RESTARTS   AGE")
		for _, pod := range report.SystemPods {
			fmt.Fprintf(&b, "%-34s %-7s %-9s %-10d %s\n",
				pod.Name, pod.Ready, pod.Status, pod.Restarts, pod.Age)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "Epinio version: %s\n", report.EpinioVersion)
	fmt.Fprintf(&b, "Kubernetes version: %s\n", report.KubernetesVersion)
	fmt.Fprintf(&b, "Platform: %s\n\n", report.Platform)

	fmt.Fprintln(&b, "Cluster Id   Name     K8s Version            Provider   Created                Nodes")
	for _, cluster := range report.Clusters {
		fmt.Fprintf(&b, "%-11s %-8s %-21s %-10s %-21s %d\n",
			cluster.ID, cluster.Name, cluster.KubeVersion, cluster.Provider, cluster.CreatedAt, cluster.NodeCount)
	}
	fmt.Fprintln(&b)

	for _, cluster := range report.Clusters {
		fmt.Fprintln(&b, "--------------------------------------------------------------------------------")
		fmt.Fprintf(&b, "Cluster: %s (%s)\n", cluster.Name, cluster.ID)
		fmt.Fprintln(&b, "Node Id         Address                                                                  etcd    Control Plane   Worker   CPU   RAM         OS                             Container Runtime Version   Created")
		for _, node := range cluster.Nodes {
			fmt.Fprintf(&b, "%-14s %-71s %-7t %-15t %-8t %-5s %-11s %-30s %-27s %s\n",
				node.ID,
				node.Address,
				node.Etcd,
				node.ControlPlane,
				node.Worker,
				node.CPU,
				node.RAM,
				node.OS,
				node.ContainerRuntimeVersion,
				node.CreatedAt,
			)
		}
		fmt.Fprintf(&b, "Node count: %d\n\n", cluster.NodeCount)
	}

	if len(report.Applications) > 0 {
		fmt.Fprintln(&b, "Scaling summary")
		fmt.Fprintln(&b, "Namespace   App                 Created                Desired   Last scale")
		for _, app := range report.Applications {
			lastScale := "n/a"
			if app.LastScaleAt != "" {
				lastScale = fmt.Sprintf("%s (%d->%d by %s)", app.LastScaleAt, app.LastScaleFrom, app.LastScaleTo, app.LastScaleBy)
			}
			fmt.Fprintf(&b, "%-11s %-19s %-21s %-8d %s\n",
				app.Namespace,
				app.Name,
				app.CreatedAt,
				app.DesiredInstances,
				lastScale,
			)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "--------------------------------------------------------------------------------")
	fmt.Fprintf(&b, "Total node count: %d\n", report.TotalNodeCount)

	return b.String()
}

type scaleEventInfo struct {
	timestamp time.Time
	from      int32
	to        int32
	username  string
}

func findLatestScaleEvent(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) *scaleEventInfo {
	selector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=App", appRef.Name)
	events, err := cluster.Kubectl.CoreV1().Events(appRef.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: selector,
	})
	if err != nil {
		helpers.Logger.Infow("failed to list app events for report", "error", err, "app", appRef.Name, "namespace", appRef.Namespace)
		return nil
	}

	var latest *scaleEventInfo
	for _, event := range events.Items {
		if !strings.HasPrefix(event.Reason, "Scale") {
			continue
		}
		timestamp := eventTimestamp(event)
		from, to, user, ok := parseScaleMessage(event.Message)
		if !ok {
			continue
		}

		info := scaleEventInfo{
			timestamp: timestamp,
			from:      from,
			to:        to,
			username:  user,
		}

		if latest == nil || info.timestamp.After(latest.timestamp) {
			latest = &info
		}
	}

	return latest
}

func eventTimestamp(event corev1.Event) time.Time {
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	return time.Time{}
}

func parseScaleMessage(message string) (int32, int32, string, bool) {
	message = strings.TrimSpace(message)
	base := ""
	user := ""
	parts := strings.SplitN(message, " by ", 2)
	if len(parts) == 2 {
		base = parts[0]
		user = strings.TrimSpace(parts[1])
	} else if strings.HasSuffix(message, " by") {
		base = strings.TrimSuffix(message, " by")
		user = ""
	} else {
		return 0, 0, "", false
	}

	var from, to int32
	_, err := fmt.Sscanf(base, "scaled from %d to %d", &from, &to)
	if err != nil {
		return 0, 0, "", false
	}

	if user == "" {
		user = "unknown"
	}

	return from, to, user, true
}
