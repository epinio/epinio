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

package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	kubeconfig "github.com/epinio/epinio/helpers/kubernetes/config"
	generic "github.com/epinio/epinio/helpers/kubernetes/platform/generic"
	ibm "github.com/epinio/epinio/helpers/kubernetes/platform/ibm"
	k3s "github.com/epinio/epinio/helpers/kubernetes/platform/k3s"
	kind "github.com/epinio/epinio/helpers/kubernetes/platform/kind"
	minikube "github.com/epinio/epinio/helpers/kubernetes/platform/minikube"
	"github.com/epinio/epinio/internal/cli/termui"

	apibatchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	restclient "k8s.io/client-go/rest"

	// https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// APISGroupName is the api name used for epinio
	APISGroupName = "epinio.io"
)

var (
	EpinioNamespaceLabelKey         = "app.kubernetes.io/component"
	EpinioNamespaceLabelValue       = "epinio-namespace"
	EpinioAPISecretLabelKey         = fmt.Sprintf("%s/%s", APISGroupName, "api-user-credentials")
	EpinioAPISecretLabelValue       = "true"
	EpinioAPIGitCredentialsLabelKey = fmt.Sprintf("%s/%s", APISGroupName, "api-git-credentials")
	EpinioAPISecretRoleLabelKey     = fmt.Sprintf("%s/%s", APISGroupName, "role")
	EpinioAPIExportRegistryLabelKey = fmt.Sprintf("%s/%s", APISGroupName, "api-export-registry")

	EpinioAPIConfigMapRolesLabelKey   = fmt.Sprintf("%s/%s", APISGroupName, "role")
	EpinioAPISecretRolesAnnotationKey = fmt.Sprintf("%s/%s", APISGroupName, "roles")
)

// Memoization of GetCluster
var clusterMemo *Cluster

type Platform interface {
	Detect(context.Context, *kubernetes.Clientset) bool
	Describe() string
	String() string
	Load(context.Context, *kubernetes.Clientset) error
	ExternalIPs() []string
}

var SupportedPlatforms = []Platform{
	kind.NewPlatform(),
	k3s.NewPlatform(),
	ibm.NewPlatform(),
	minikube.NewPlatform(),
}

type Cluster struct {
	//	InternalIPs []string
	//	Ingress     bool
	Kubectl    *kubernetes.Clientset
	RestConfig *restclient.Config
	platform   Platform
}

// GetHTTP1Client returns a clientset that is always using HTTP/1.1 (not HTTP2)
// We need that when using the SPDY protocol and UPGRADE requests which HTTP2
// doesn't understand:
// https://github.com/golang/net/blob/183a9ca12b87817e0ced91cdd139606cbb193ef2/http2/transport.go#L1083-L1085
func GetHTTP1Client(ctx context.Context) (*kubernetes.Clientset, error) {
	restConfig, err := kubeconfig.KubeConfig()
	if err != nil {
		return nil, err
	}

	// copy to avoid mutating the passed-in config
	config := restclient.CopyConfig(restConfig)
	// set the warning handler for this client to ignore warnings
	config.WarningHandler = restclient.NoWarnings{}

	config.NextProtos = []string{"http/1.1"}

	return kubernetes.NewForConfig(config)
}

// GetCluster returns the Cluster needed to talk to it. On first call it
// creates it from a Kubernetes rest client config and cli arguments /
// environment variables.
func GetCluster(ctx context.Context) (*Cluster, error) {
	if clusterMemo != nil {
		return clusterMemo, nil
	}

	c := &Cluster{}

	restConfig, err := kubeconfig.KubeConfig()
	if err != nil {
		return nil, err
	}

	// copy to avoid mutating the passed-in config
	config := restclient.CopyConfig(restConfig)
	// set the warning handler for this client to ignore warnings
	config.WarningHandler = restclient.NoWarnings{}

	c.RestConfig = restConfig
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	c.Kubectl = clientset
	c.detectPlatform(ctx)
	if c.platform == nil {
		c.platform = generic.NewPlatform()
	}

	err = c.platform.Load(ctx, clientset)
	if err != nil {
		return nil, err
	}

	clusterMemo = c

	return clusterMemo, nil
}

func (c *Cluster) GetPlatform() Platform {
	return c.platform
}

func (c *Cluster) detectPlatform(ctx context.Context) {
	for _, p := range SupportedPlatforms {
		if p.Detect(ctx, c.Kubectl) {
			c.platform = p
			return
		}
	}
}

// ClientAppChart returns a dynamic client for the app chart resource
func (c *Cluster) ClientAppChart() (dynamic.NamespaceableResourceInterface, error) {
	cs, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    "application.epinio.io",
		Version:  "v1",
		Resource: "appcharts",
	}
	return cs.Resource(gvr), nil
}

// ClientApp returns a dynamic namespaced client for the app resource
func (c *Cluster) ClientApp() (dynamic.NamespaceableResourceInterface, error) {
	cs, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    "application.epinio.io",
		Version:  "v1",
		Resource: "apps",
	}
	return cs.Resource(gvr), nil
}

// IsJobFailed is a condition function that indicates whether the
// given Job is in Failed state or not.
func (c *Cluster) IsJobFailed(ctx context.Context, jobName, namespace string) (bool, error) {
	client, err := typedbatchv1.NewForConfig(c.RestConfig)
	if err != nil {
		return false, err
	}

	job, err := client.Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == apibatchv1.JobFailed && condition.Status == v1.ConditionTrue {
			return true, nil
		}
	}
	return false, nil
}

// IsJobDone returns a condition function that indicates whether the given
// Job is done (Completed or Failed), or not
func (c *Cluster) IsJobDone(ctx context.Context, client *typedbatchv1.BatchV1Client, jobName, namespace string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		job, err := client.Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range job.Status.Conditions {
			if condition.Status == v1.ConditionTrue &&
				(condition.Type == apibatchv1.JobFailed ||
					condition.Type == apibatchv1.JobComplete) {
				return true, nil
			}
		}
		return false, nil
	}
}

func (c *Cluster) NamespaceDoesNotExist(ctx context.Context, namespaceName string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		exists, err := c.NamespaceExists(ctx, namespaceName)
		return !exists, err
	}
}

func (c *Cluster) PodDoesNotExist(ctx context.Context, namespace, selector string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		podList, err := c.ListPods(ctx, namespace, selector)
		if err != nil {
			return true, nil
		}
		if len(podList.Items) > 0 {
			return false, nil
		}
		return true, nil
	}
}

// WaitForSecret waits until the specified secret exists. If timeout is reached,
// an error is returned.
// It should be used when something is expected to create a Secret and the code
// needs to wait until that happens.
func (c *Cluster) WaitForSecret(ctx context.Context, namespace, secretName string, timeout time.Duration) (*v1.Secret, error) {
	var secret *v1.Secret
	waitErr := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		var err error
		secret, err = c.GetSecret(ctx, namespace, secretName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})

	return secret, waitErr
}

func (c *Cluster) WaitForJobDone(ctx context.Context, namespace, jobName string, timeout time.Duration) error {
	client, err := typedbatchv1.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, c.IsJobDone(ctx, client, jobName, namespace))
}

// ListPods returns the list of currently scheduled or running pods in `namespace` with the given selector
func (c *Cluster) ListPods(ctx context.Context, namespace, selector string) (*v1.PodList, error) {
	listOptions := metav1.ListOptions{}
	if len(selector) > 0 {
		listOptions.LabelSelector = selector
	}
	podList, err := c.Kubectl.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// ListJobs returns the list of currently scheduled or running Jobs in `namespace` with the given selector
func (c *Cluster) ListJobs(ctx context.Context, namespace, selector string) (*apibatchv1.JobList, error) {
	listOptions := metav1.ListOptions{}
	if len(selector) > 0 {
		listOptions.LabelSelector = selector
	}
	jobList, err := c.Kubectl.BatchV1().Jobs(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return jobList, nil
}

func (c *Cluster) CreateJob(ctx context.Context, namespace string, job *apibatchv1.Job) error {
	_, err := c.Kubectl.BatchV1().Jobs(namespace).Create(
		ctx,
		job,
		metav1.CreateOptions{},
	)
	return err
}

// DeleteJob deletes the namepace
func (c *Cluster) DeleteJob(ctx context.Context, namespace string, name string) error {
	policy := metav1.DeletePropagationBackground
	return c.Kubectl.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
}

// Wait up to timeout for Namespace to be removed.
// Returns an error if the Namespace is not removed within the allotted time.
func (c *Cluster) WaitForNamespaceMissing(ctx context.Context, ui *termui.UI, namespace string, timeout time.Duration) error {
	if ui != nil {
		s := ui.Progressf("Waiting for namespace %s to be deleted", namespace)
		defer s.Stop()
	}

	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, c.NamespaceDoesNotExist(ctx, namespace))
}

// Wait up to timeout for pod to be removed.
// Returns an error if the pod is not removed within the allotted time.
func (c *Cluster) WaitForPodBySelectorMissing(ctx context.Context, namespace, selector string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, c.PodDoesNotExist(ctx, namespace, selector))
}

// GetConfigMap gets a configmap's values
func (c *Cluster) GetConfigMap(ctx context.Context, namespace, name string) (*v1.ConfigMap, error) {
	return c.Kubectl.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetSecret gets a secret's values
func (c *Cluster) GetSecret(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	return c.Kubectl.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// DeleteSecret removes a secret
func (c *Cluster) DeleteSecret(ctx context.Context, namespace, name string) error {
	err := c.Kubectl.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to delete secret")
	}

	return nil
}

// CreateSecret posts the specified secret to the cluster. All
// configuration of the secret is done by the caller.
func (c *Cluster) CreateSecret(ctx context.Context, namespace string, secret v1.Secret) error {
	_, err := c.Kubectl.CoreV1().Secrets(namespace).Create(ctx,
		&secret,
		metav1.CreateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to create secret %s", secret.Name)
	}
	return nil
}

// CreateLabeledSecret posts a new secret to the cluster. The secret
// is constructed from name and a key/value dictionary for labels.
func (c *Cluster) CreateLabeledSecret(ctx context.Context, namespace, name string,
	data map[string][]byte,
	label map[string]string,
	annotations map[string]string,
) error {

	secret := &v1.Secret{
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      label,
			Annotations: annotations,
		},
	}
	_, err := c.Kubectl.CoreV1().Secrets(namespace).Create(ctx,
		secret,
		metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create secret")
	}

	return nil
}

// GetVersion get the kube server version
func (c *Cluster) GetVersion() (string, error) {
	v, err := c.Kubectl.ServerVersion()
	if err != nil {
		return "", errors.Wrap(err, "failed to get kube server version")
	}

	return v.String(), nil
}

// ListIngress returns the list of available ingresses in `namespace` with the given selector
func (c *Cluster) ListIngress(ctx context.Context, namespace, selector string) (*networkingv1.IngressList, error) {
	listOptions := metav1.ListOptions{}
	if len(selector) > 0 {
		listOptions.LabelSelector = selector
	}

	// TODO: Switch to networking v1 when we don't care about <1.18 clusters
	ingressList, err := c.Kubectl.NetworkingV1().Ingresses(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	return ingressList, nil
}

// NamespaceExists checks if a namespace exists or not
func (c *Cluster) NamespaceExists(ctx context.Context, namespaceName string) (bool, error) {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
