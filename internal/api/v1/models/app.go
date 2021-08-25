package models

import (
	"context"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/names"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	EpinioStageIDLabel = "epinio.suse.org/stage-id"
)

// App has all the app properties, like the routes and stage ID.
// It is used in the CLI and  API responses.
type App struct {
	Active        bool     `json:"active,omitempty"`
	StageID       string   `json:"stage_id,omitempty"`
	Name          string   `json:"name,omitempty"`
	Organization  string   `json:"namespace,omitempty"`
	Username      string   `json:"username,omitempty"`
	Status        string   `json:"status,omitempty"`
	Route         string   `json:"route,omitempty"`
	BoundServices []string `json:"bound_services,omitempty"`
}

// NewApp returns a new app for name and org
func NewApp(name string, org string) *App {
	return &App{Name: name, Organization: org}
}

// AppRef returns a reference to the app (name, org)
func (a *App) AppRef() AppRef {
	return NewAppRef(a.Name, a.Organization)
}

// AppList is a collection of app references
type AppList []App

// Implement the Sort interface for application slices

// Len (Sort interface) returns the length of the AppList
func (al AppList) Len() int {
	return len(al)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the AppList
func (al AppList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the AppList and returns true if the condition holds, and
// else false.
func (al AppList) Less(i, j int) bool {
	return al[i].Name < al[j].Name
}

// AppRef references an App by name and org
type AppRef struct {
	Name string `json:"name"`
	Org  string `json:"namespace"`
}

// NewAppRef returns a new reference to an app
func NewAppRef(name string, org string) AppRef {
	return AppRef{Name: name, Org: org}
}

// App returns an fresh app model for the reference
func (ar *AppRef) App() *App {
	return NewApp(ar.Name, ar.Org)
}

// EnvSecret returns the name of the kube secret holding the
// environment variables of the referenced application
func (ar *AppRef) EnvSecret() string {
	// TODO: This needs tests for env operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-env")
}

// PVCName returns the name of the kube pvc to use with/for the referenced application.
func (ar *AppRef) PVCName() string {
	return names.GenerateResourceName(ar.Org, ar.Name)
}

// EnsurePVC creates a PVC for the application if one doesn't already exist.
// This PVC is used to store the application source blobs (as they are uploaded
// on the "upload" endpoint). It's also mounted in the staging task pod as the
// "source" tekton workspace.
// The same PVC stores the application's build cache (on a separate directory).
func (ar *AppRef) EnsurePVC(ctx context.Context, cluster *kubernetes.Cluster) error {
	_, err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(deployments.TektonStagingNamespace).
		Get(ctx, ar.PVCName(), metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) { // Unknown error, irrelevant to non-existence
		return err
	}
	if err == nil { // pvc already exists
		return nil
	}

	// From here on, only if the PVC is missing
	_, err = cluster.Kubectl.CoreV1().PersistentVolumeClaims(deployments.TektonStagingNamespace).
		Create(ctx, &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ar.PVCName(),
				Namespace: deployments.TektonStagingNamespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}, metav1.CreateOptions{})

	return err
}

// StageRef references a tekton staging run by ID, currently randomly generated
// for each POST to the staging endpoint
type StageRef struct {
	ID string `json:"id,omitempty"`
}

// NewStage returns a new reference to a staging run
func NewStage(id string) StageRef {
	return StageRef{id}
}

// ImageRef references an upload
type ImageRef struct {
	ID string `json:"id,omitempty"`
}

// NewImage returns a new image ref for the given ID
func NewImage(id string) ImageRef {
	return ImageRef{id}
}
