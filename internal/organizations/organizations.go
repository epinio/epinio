// Package organizations encapsulates all the functionality around Epinio-controlled namespaces
// TODO: Consider moving this + the applications + the services packages under
// "models".
package organizations

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/duration"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Organization represents an epinio-controlled namespace in the system
type Organization struct {
	Name string
}

func List(ctx context.Context, kubeClient *kubernetes.Cluster) ([]Organization, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: kubernetes.EpinioOrgLabelKey + "=" + kubernetes.EpinioOrgLabelValue,
	}

	orgList, err := kubeClient.Kubectl.CoreV1().Namespaces().List(ctx, listOptions)
	if err != nil {
		return []Organization{}, err
	}

	result := []Organization{}
	for _, org := range orgList.Items {
		result = append(result, Organization{Name: org.ObjectMeta.Name})
	}

	return result, nil
}

// Exists checks if the named epinio-controlled namespace exists or
// not, and returns an appropriate boolean flag
func Exists(ctx context.Context, kubeClient *kubernetes.Cluster, lookupOrg string) (bool, error) {
	orgs, err := List(ctx, kubeClient)
	if err != nil {
		return false, err
	}
	for _, org := range orgs {
		if org.Name == lookupOrg {
			return true, nil
		}
	}

	return false, nil
}

// Create generates a new epinio-controlled namespace, i.e. a kube
// namespace plus a service account.
func Create(ctx context.Context, kubeClient *kubernetes.Cluster, org string) error {
	if _, err := kubeClient.Kubectl.CoreV1().Namespaces().Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: org,
				Labels: map[string]string{
					"kubed-sync":                 "registry-creds", // Let kubed copy-over image pull secrets
					kubernetes.EpinioOrgLabelKey: kubernetes.EpinioOrgLabelValue,
				},
				Annotations: map[string]string{
					"linkerd.io/inject": "enabled",
				},
			},
		},
		metav1.CreateOptions{},
	); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return errors.Errorf("Namespace '%s' name cannot be used. Please try another name", org)
		}
		return err
	}

	if err := createServiceAccount(ctx, kubeClient, org); err != nil {
		return errors.Wrap(err, "failed to create a service account for apps")
	}

	return nil
}

// Delete destroys an epinio-controlled namespace, i.e. the associated
// kube namespace and service account.
func Delete(ctx context.Context, kubeClient *kubernetes.Cluster, org string) error {
	err := kubeClient.Kubectl.CoreV1().Namespaces().Delete(ctx, org, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return kubeClient.WaitForNamespaceMissing(ctx, nil, org, duration.ToOrgDeletion())
}

// createServiceAccount is a helper to `Create` which creates the
// service account applications pushed to the namespace need for
// permission handling.
func createServiceAccount(ctx context.Context, kubeClient *kubernetes.Cluster, targetOrg string) error {
	automountServiceAccountToken := true
	_, err := kubeClient.Kubectl.CoreV1().ServiceAccounts(targetOrg).Create(
		ctx,
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: targetOrg,
			},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "registry-creds"},
			},
			AutomountServiceAccountToken: &automountServiceAccountToken,
		}, metav1.CreateOptions{})

	return err
}
