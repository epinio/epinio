package application

import (
	"context"
	"encoding/json"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// Origin returns the origin of the specified application. The data is
// constructed from the stored information on the Application Custom
// Resource.
func Origin(app *unstructured.Unstructured) (models.ApplicationOrigin, error) {
	result := models.ApplicationOrigin{
		Git: &models.GitRef{},
	}

	origin, found, err := unstructured.NestedMap(app.Object, "spec", "origin")

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
			return result, errors.New("bad path origin, empty string")
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
			return result, errors.New("bad container origin, empty string")
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
			return result, errors.New("bad git origin, url is empty string")
		}

		// For git check for the optional revision as well.
		revision, found, err := unstructured.NestedString(origin, "git", "revision")
		if found {
			if err != nil {
				return result, err
			}
			if revision == "" {
				return result, errors.New("bad git origin, revision is empty string")
			}
			result.Git.Revision = revision
		}

		result.Kind = models.OriginGit
		result.Git.URL = repository
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

	patch, err := buildBodyPatch(origin)
	if err != nil {
		return errors.Wrap(err, "error building body patch")
	}

	_, err = client.Namespace(app.Namespace).Patch(ctx,
		app.Name,
		types.JSONPatchType,
		patch,
		metav1.PatchOptions{})

	return err
}

func buildBodyPatch(origin models.ApplicationOrigin) ([]byte, error) {
	operations := []PatchOperation{{
		Op:    "replace",
		Path:  "/spec/origin",
		Value: origin,
	}}

	return json.Marshal(operations)
}

type PatchOperation struct {
	Op    string                   `json:"op"`
	Path  string                   `json:"path"`
	Value models.ApplicationOrigin `json:"value"`
}
