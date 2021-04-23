package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pkg/errors"

	kubeconfig "github.com/epinio/epinio/helpers/kubernetes/config"
	generic "github.com/epinio/epinio/helpers/kubernetes/platform/generic"
	ibm "github.com/epinio/epinio/helpers/kubernetes/platform/ibm"
	k3s "github.com/epinio/epinio/helpers/kubernetes/platform/k3s"
	kind "github.com/epinio/epinio/helpers/kubernetes/platform/kind"
	minikube "github.com/epinio/epinio/helpers/kubernetes/platform/minikube"
	"github.com/epinio/epinio/helpers/termui"

	apibatchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	// https://github.com/kubernetes/client-go/issues/345
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// APISGroupName is the api name used for epinio
	APISGroupName = "epinio.suse.org"
)

var (
	EpinioDeploymentLabelKey   = fmt.Sprintf("%s/%s", APISGroupName, "deployment")
	EpinioDeploymentLabelValue = "true"
)

// Memoization of GetCluster
var clusterMemo *Cluster

type Platform interface {
	Detect(*kubernetes.Clientset) bool
	Describe() string
	String() string
	Load(*kubernetes.Clientset) error
	ExternalIPs() []string
}

var SupportedPlatforms []Platform = []Platform{
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

// GetCluster returns the Cluster needed to talk to it. On first call it
// creates it from a Kubernetes rest client config and cli arguments /
// environment variables.
func GetCluster() (*Cluster, error) {
	if clusterMemo != nil {
		return clusterMemo, nil
	}

	c := &Cluster{}

	restConfig, err := kubeconfig.KubeConfig()
	if err != nil {
		return nil, err
	}

	c.RestConfig = restConfig
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	c.Kubectl = clientset
	c.detectPlatform()
	if c.platform == nil {
		c.platform = generic.NewPlatform()
	}

	err = c.platform.Load(clientset)
	if err != nil {
		return nil, err
	}

	clusterMemo = c

	return clusterMemo, nil
}

func (c *Cluster) GetPlatform() Platform {
	return c.platform
}

func (c *Cluster) detectPlatform() {
	for _, p := range SupportedPlatforms {
		if p.Detect(c.Kubectl) {
			c.platform = p
			return
		}
	}
}

// IsPodRunning returns a condition function that indicates whether the given pod is
// currently running
func (c *Cluster) IsPodRunning(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := c.Kubectl.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}
}

// IsJobCompleted returns a condition function that indicates whether the given
// Job is in Completed state.
func (c *Cluster) IsJobCompleted(client *typedbatchv1.BatchV1Client, jobName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		job, err := client.Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, condition := range job.Status.Conditions {
			if condition.Type == apibatchv1.JobComplete && condition.Status == v1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	}
}

func (c *Cluster) PodExists(namespace, selector string) wait.ConditionFunc {
	return func() (bool, error) {
		podList, err := c.ListPods(namespace, selector)
		if err != nil {
			return false, err
		}
		if len(podList.Items) == 0 {
			return false, nil
		}
		return true, nil
	}
}

func (c *Cluster) NamespaceDoesNotExist(namespaceName string) wait.ConditionFunc {
	return func() (bool, error) {
		exists, err := c.NamespaceExists(namespaceName)
		return !exists, err
	}
}

func (c *Cluster) PodDoesNotExist(namespace, selector string) wait.ConditionFunc {
	return func() (bool, error) {
		podList, err := c.ListPods(namespace, selector)
		if err != nil {
			return true, nil
		}
		if len(podList.Items) > 0 {
			return false, nil
		}
		return true, nil
	}
}

// WaitForCRD wait for a custom resource definition to exist in the cluster.
// This method should be used when installing a Deployment that is supposed to
// provide that CRD and want to make sure the CRD is ready for consumption before
// continuing deploying things that will consume it.
func (c *Cluster) WaitForCRD(ui *termui.UI, CRDName string, timeout time.Duration) error {
	s := ui.Progressf("Waiting for CRD %s to be ready to use", CRDName)
	defer s.Stop()

	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		clientset, err := apiextensions.NewForConfig(c.RestConfig)
		if err != nil {
			return false, err
		}

		_, err = clientset.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), CRDName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			} else {
				return false, err
			}
		}

		return true, nil
	})
}

// WaitForSecret waits until the specified secret exists. If timeout is reached,
// an error is returned.
// It should be used when something is expected to create a Secret and the code
// needs to wait until that happens.
func (c *Cluster) WaitForSecret(namespace, secretName string, timeout time.Duration) (*v1.Secret, error) {
	var secret *v1.Secret
	waitErr := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		var err error
		secret, err = c.GetSecret(namespace, secretName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			} else {
				return false, err
			}
		}
		return true, nil
	})

	return secret, waitErr
}

// Poll up to timeout for pod to enter running state.
// Returns an error if the pod never enters the running state.
func (c *Cluster) WaitForPodRunning(namespace, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, c.IsPodRunning(podName, namespace))
}

func (c *Cluster) WaitForJobCompleted(namespace, jobName string, timeout time.Duration) error {
	client, err := typedbatchv1.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}
	return wait.PollImmediate(time.Second, timeout, c.IsJobCompleted(client, jobName, namespace))
}

// ListPods returns the list of currently scheduled or running pods in `namespace` with the given selector
func (c *Cluster) ListPods(namespace, selector string) (*v1.PodList, error) {
	listOptions := metav1.ListOptions{}
	if len(selector) > 0 {
		listOptions.LabelSelector = selector
	}
	podList, err := c.Kubectl.CoreV1().Pods(namespace).List(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// Wait up to timeout for Namespace to be removed.
// Returns an error if the Namespace is not removed within the allotted time.
func (c *Cluster) WaitForNamespaceMissing(ui *termui.UI, namespace string, timeout time.Duration) error {
	s := ui.Progressf("Waiting for namespace %s to be deleted", namespace)
	defer s.Stop()

	return wait.PollImmediate(time.Second, timeout, c.NamespaceDoesNotExist(namespace))
}

// Wait up to timeout for pod to be removed.
// Returns an error if the pod is not removed within the allotted time.
func (c *Cluster) WaitForPodBySelectorMissing(ui *termui.UI, namespace, selector string, timeout time.Duration) error {
	if ui != nil {
		s := ui.Progressf("Removing %s in %s", selector, namespace)
		defer s.Stop()
	}

	return wait.PollImmediate(time.Second, timeout, c.PodDoesNotExist(namespace, selector))
}

// Wait up to timeout for all pods in 'namespace' with given 'selector' to enter running state.
// Returns an error if no pods are found or not all discovered pods enter running state.
func (c *Cluster) WaitUntilPodBySelectorExist(ui *termui.UI, namespace, selector string, timeout time.Duration) error {
	s := ui.Progressf("Creating %s in %s", selector, namespace)
	defer s.Stop()

	return wait.PollImmediate(time.Second, timeout, c.PodExists(namespace, selector))
}

// WaitForPodBySelectorRunning waits timeout for all pods in 'namespace'
// with given 'selector' to enter running state. Returns an error if no pods are
// found or not all discovered pods enter running state.
func (c *Cluster) WaitForPodBySelectorRunning(ui *termui.UI, namespace, selector string, timeout time.Duration) error {
	s := ui.Progressf("Starting %s in %s", selector, namespace)
	defer s.Stop()

	podList, err := c.ListPods(namespace, selector)
	if err != nil {
		return errors.Wrapf(err, "failed listingpods with selector %s", selector)
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods in %s with selector %s", namespace, selector)
	}

	for _, pod := range podList.Items {
		s.ChangeMessagef("  Starting pod %s in %s", pod.Name, namespace)
		if err := c.WaitForPodRunning(namespace, pod.Name, timeout); err != nil {
			events, err2 := c.GetPodEvents(namespace, pod.Name)
			if err2 != nil {
				return errors.Wrap(err, err2.Error())
			} else {
				return errors.New(fmt.Sprintf("Failed waiting for %s: %s\nPod Events: \n%s", pod.Name, err.Error(), events))
			}
		}
	}
	return nil
}

// GetPodEventsWithSelector tries to find a pod using the provided selector and
// namespace. If found it returns the events on that Pod. If not found it returns
// an error.
// An equivalent kubectl command would look like this
// (label selector being "app.kubernetes.io/name=container-registry"):
//   kubectl get event --namespace my-namespace \
//   --field-selector involvedObject.name=$( \
//     kubectl get pods -o=jsonpath='{.items[0].metadata.name}' --selector=app.kubernetes.io/name=container-registry -n my-namespace)
func (c *Cluster) GetPodEventsWithSelector(namespace, selector string) (string, error) {
	podList, err := c.Kubectl.CoreV1().Pods(namespace).List(context.Background(),
		metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", err
	}
	if len(podList.Items) < 1 {
		return "", errors.New(fmt.Sprintf("Couldn't find Pod with selector '%s' in namespace %s", selector, namespace))
	}
	podName := podList.Items[0].Name

	return c.GetPodEvents(namespace, podName)
}

func (c *Cluster) GetPodEvents(namespace, podName string) (string, error) {
	eventList, err := c.Kubectl.CoreV1().Events(namespace).List(context.Background(),
		metav1.ListOptions{
			FieldSelector: "involvedObject.name=" + podName,
		})
	if err != nil {
		return "", err
	}

	events := []string{}
	for _, event := range eventList.Items {
		events = append(events, event.Message)
	}

	return strings.Join(events, "\n"), nil
}

func (c *Cluster) Exec(namespace, podName, containerName string, command, stdin string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	stdinput := bytes.NewBuffer([]byte(stdin))

	err := c.execPod(namespace, podName, containerName, command, stdinput, &stdout, &stderr)

	// if options.PreserveWhitespace {
	// 	return stdout.String(), stderr.String(), err
	// }
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

// GetSecret gets a secret's values
func (c *Cluster) GetSecret(namespace, name string) (*v1.Secret, error) {
	return c.Kubectl.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// DeleteSecret removes a secret
func (c *Cluster) DeleteSecret(namespace, name string) error {
	err := c.Kubectl.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to delete secret")
	}

	return nil
}

// CreateSecret posts a new secret with key/value dictionary.
func (c *Cluster) CreateSecret(namespace, name string, data map[string][]byte) error {
	secret := &v1.Secret{
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := c.Kubectl.CoreV1().Secrets(namespace).Create(context.Background(),
		secret,
		metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create secret")
	}
	return nil
}

// CreateLabeledSecret posts a new secret with key/value dictionary.
func (c *Cluster) CreateLabeledSecret(namespace, name string,
	data map[string][]byte,
	label map[string]string) error {

	secret := &v1.Secret{
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := c.Kubectl.CoreV1().Secrets(namespace).Create(context.Background(),
		secret,
		metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to create secret")
	}

	// FIXME ... We patch the labels in ... Easier than trying to
	// find all the necessary types for the Secret structure.

	return c.LabelSecret(namespace, name, label)
}

// LabelSecret patches a secret with labels. Analogous to
// LabelNamespace later in this file.
func (c *Cluster) LabelSecret(
	namespace, name string,
	label map[string]string) error {

	labels := []string{}
	for key, value := range label {
		labels = append(labels, fmt.Sprintf(`"%s":"%s"`, key, value))
	}

	patchContents := fmt.Sprintf(`{ "metadata": { "labels": { %s } } }`,
		strings.Join(labels, ","))

	_, err := c.Kubectl.CoreV1().Secrets(namespace).Patch(context.Background(), name,
		types.StrategicMergePatchType, []byte(patchContents), metav1.PatchOptions{})

	if err != nil {
		return err
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

// DeploymentStatus returns running status for a Deployment
// If the deployment doesn't exist, the status is set to 0/0
func (c *Cluster) DeploymentStatus(namespace, selector string) (string, error) {
	result, err := c.Kubectl.AppsV1().Deployments(namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: selector,
		},
	)

	if err != nil {
		return "", errors.Wrap(err, "failed to get Deployment status")
	}

	if len(result.Items) < 1 {
		return "0/0", nil
	}

	return fmt.Sprintf("%d/%d", result.Items[0].Status.ReadyReplicas, result.Items[0].Status.Replicas), nil
}

// ListIngressRoutes returns a list of all routes for ingresses in `namespace` with the given selector
func (c *Cluster) ListIngressRoutes(namespace, name string) ([]string, error) {
	ingress, err := c.Kubectl.NetworkingV1().Ingresses(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list ingresses")
	}

	result := []string{}

	for _, rule := range ingress.Spec.Rules {
		result = append(result, rule.Host)
	}

	return result, nil
}

// ListIngress returns the list of available ingresses in `namespace` with the given selector
func (c *Cluster) ListIngress(namespace, selector string) (*v1beta1.IngressList, error) {
	listOptions := metav1.ListOptions{}
	if len(selector) > 0 {
		listOptions.LabelSelector = selector
	}

	// TODO: Switch to networking v1 when we don't care about <1.18 clusters
	ingressList, err := c.Kubectl.ExtensionsV1beta1().Ingresses(namespace).List(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}
	return ingressList, nil
}

func (c *Cluster) execPod(namespace, podName, containerName string,
	command string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := c.Kubectl.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
		Namespace(namespace).SubResource("exec")
	option := &v1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}
	if stdin == nil {
		option.Stdin = false
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(c.RestConfig, "POST", req.URL())
	if err != nil {
		return err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return err
	}

	return nil
}

// LabelNamespace adds a label to the namespace
func (c *Cluster) LabelNamespace(namespace, labelKey, labelValue string) error {
	patchContents := fmt.Sprintf(`{ "metadata": { "labels": { "%s": "%s" } } }`, labelKey, labelValue)

	_, err := c.Kubectl.CoreV1().Namespaces().Patch(context.Background(), namespace,
		types.StrategicMergePatchType, []byte(patchContents), metav1.PatchOptions{})

	if err != nil {
		return err
	}

	return nil
}

// NamespaceExistsAndOwned checks if the namespace exists
// and is created by epinio or not.
func (c *Cluster) NamespaceExistsAndOwned(namespaceName string) (bool, error) {
	exists, err := c.NamespaceExists(namespaceName)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	owned, err := c.NamespaceLabelExists(namespaceName, EpinioDeploymentLabelKey)
	if err != nil {
		return false, err
	}
	return owned, nil
}

// NamespaceExists checks if a namespace exists or not
func (c *Cluster) NamespaceExists(namespaceName string) (bool, error) {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(context.Background(), namespaceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// NamespaceLabelExists checks if a specific label exits on the namespace
func (c *Cluster) NamespaceLabelExists(namespaceName, labelKey string) (bool, error) {
	namespace, err := c.Kubectl.CoreV1().Namespaces().Get(context.Background(), namespaceName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	labelKey = fmt.Sprintf("%s/%s", APISGroupName, "deployment")
	if _, found := namespace.GetLabels()[labelKey]; found {
		return true, nil
	}

	return false, nil
}

// DeleteNamespace deletes the namepace
func (c *Cluster) DeleteNamespace(namespace string) error {
	err := c.Kubectl.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
