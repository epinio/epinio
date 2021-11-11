package application

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// Origin returns the origin of the specified application. The data is
// constructed from the stored information on the Application Custom
// Resource.
func Origin(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (models.ApplicationOrigin, error) {
	result := models.ApplicationOrigin{}

	applicationCR, err := Get(ctx, cluster, appRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return result, apierror.AppIsNotKnown("application resource is missing")
		}
		return result, apierror.InternalError(err, "failed to get the application resource")
	}

	origin, found, err := unstructured.NestedMap(applicationCR.Object, "spec", "origin")

	if !found {
		return result, nil
	}
	if err != nil {
		return result, err
	}

	// Check, in order, for `path`, `container`, and `git` origins
	// Notes:
	//   - Only one of path, container, or git may be present.
	//     If more than one is present the first found is taken
	//     (See above for the order)
	//   - If a value is present it must not be empty.
	//     IOW spec.origin.path == "", etc. are rejected.

	path, found, err := unstructured.NestedString(origin, "path")
	if found {
		if err != nil {
			return result, err
		}
		if path == "" {
			return result, apierror.InternalError(err, "Bad path origin, empty string")
		}
		result.Kind = models.OriginPath
		result.Path = path
		return result, nil
	}

	container, found, err := unstructured.NestedString(origin, "container")
	if found {
		if err != nil {
			return result, err
		}
		if container == "" {
			return result, apierror.InternalError(err, "Bad container origin, empty string")
		}
		result.Kind = models.OriginContainer
		result.Container = container
		return result, nil
	}

	repository, found, err := unstructured.NestedString(origin, "git", "repository")
	if found {
		if err != nil {
			return result, err
		}
		if repository == "" {
			return result, apierror.InternalError(err, "Bad git origin, url is empty string")
		}
		result.Kind = models.OriginGit
		result.Git.URL = repository

		// For git check for the optional revision as well.
		revision, found, err := unstructured.NestedString(origin, "git", "revision")
		if found {
			if err != nil {
				return result, err
			}
			if revision == "" {
				return result, apierror.InternalError(err, "Bad git origin, revision is empty string")
			}
			result.Git.Revision = revision
		}

		return result, nil
	}

	// Nothing found. Return as is, undefined origin. This can
	// happen for applications which were created, but not pushed
	// yet.

	return result, nil
}

// SetOrigin patches the new origin information into the specified application.
func SetOrigin(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef, origin models.ApplicationOrigin) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	// Assemble new origin json
	var value string
	switch origin.Kind {
	case models.OriginNone:
		value = ""
	case models.OriginPath:
		value = fmt.Sprintf(`"path": "%s"`, origin.Path)
	case models.OriginContainer:
		value = fmt.Sprintf(`"container": "%s"`, origin.Container)
	case models.OriginGit:
		value = fmt.Sprintf(`"repository": "%s"`, origin.Git.URL)
		if origin.Git.Revision != "" {
			value = fmt.Sprintf(`%s, "revision": "%s"`, value, origin.Git.Revision)
		}
		value = fmt.Sprintf(`"git": {%s}`, value)
	}

	// And enter into the app
	patch := fmt.Sprintf(`[{"op": "replace", "path": "/spec/origin", "value": {%s}}]`, value)

	_, err = client.Namespace(app.Namespace).Patch(ctx,
		app.Name,
		types.JSONPatchType,
		[]byte(patch),
		metav1.PatchOptions{})

	return err
}
