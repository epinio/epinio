// Package organizations encapsulates all the functionality around Epinio-controlled namespaces
// TODO: Consider moving this + the applications + the services packages under
// "models".
package organizations

import (
	"context"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
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

// GiteaInterface is the interface to whatever backend is used to
// manage epinio-controlled namespaces beyond them being kube
// namespaces. The chosen name stronly implies gitea and a client for
// it, unfortunately. See also file
// `internal/cli/clients/gitea/gitea.go`. TODO: Seek a better name.
type GiteaInterface interface {
	// Create a new epinio-controlled namespace
	CreateOrg(org string) error
	// Delete the named epinio-controlled namespace
	DeleteOrg(org string) error
}

// List returns a list of the known epinio-controlled namespaces. This
// is essentially the set of kube namespaces tagged as being under
// epinio's control.
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
// namespace plus a service account. The provided giteaInterface is
// used to create whatever other dependent (non-kube) resources are
// needed.
func Create(ctx context.Context, kubeClient *kubernetes.Cluster, gitea GiteaInterface, org string) error {
	if _, err := kubeClient.Kubectl.CoreV1().Namespaces().Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: org,
				Labels: map[string]string{
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

	// This secret is used as ImagePullSecrets for the application ServiceAccount
	// in order to allow the image to be pulled from the registry.
	if err := copySecret(ctx, "registry-creds", deployments.TektonStagingNamespace, org, kubeClient); err != nil {
		return errors.Wrap(err, "failed to copy the registry credentials secret")
	}

	if err := createServiceAccount(ctx, kubeClient, org); err != nil {
		return errors.Wrap(err, "failed to create a service account for apps")
	}

	return gitea.CreateOrg(org)
}

// Delete destroys an epinio-controlled namespace, i.e. the associated
// kube namespace and service account.  The provided giteaInterface is
// used to delete whatever other dependent (non-kube) resources are
// associated with it.
func Delete(ctx context.Context, kubeClient *kubernetes.Cluster, gitea GiteaInterface, org string) error {
	err := kubeClient.Kubectl.CoreV1().Namespaces().Delete(ctx, org, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	err = kubeClient.WaitForNamespaceMissing(ctx, nil, org, duration.ToOrgDeletion())
	if err != nil {
		return err
	}

	return gitea.DeleteOrg(org)
}

// copySecret is helper to Create which replicates the specified kube
// secret into a target namespace.
func copySecret(ctx context.Context, secretName, originOrg, targetOrg string, kubeClient *kubernetes.Cluster) error {
	log := tracelog.Logger(ctx)
	log.V(1).Info("will now copy secret", "name", secretName)
	secret, err := kubeClient.Kubectl.CoreV1().
		Secrets(originOrg).
		Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newSecret := secret.DeepCopy()
	newSecret.ObjectMeta.Namespace = targetOrg
	newSecret.ResourceVersion = ""
	newSecret.OwnerReferences = []metav1.OwnerReference{}
	log.V(2).Info("newSecret", "data", newSecret)

	_, err = kubeClient.Kubectl.CoreV1().Secrets(targetOrg).
		Create(ctx, newSecret, metav1.CreateOptions{})

	return err
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
