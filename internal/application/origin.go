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
	result := models.ApplicationOrigin{}

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

		// For path check the archive flag as well
		isarchive, found, err := unstructured.NestedBool(origin, "archive")
		if found {
			if err != nil {
				return result, err
			}

			result.Archive = isarchive
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

		result.Git = &models.GitRef{}

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

		// For git check for the optional provider as well.
		provider, found, err := unstructured.NestedString(origin, "git", "provider")
		if found {
			if err != nil {
				return result, err
			}
			if provider == "" {
				return result, errors.New("bad git origin, provider is empty string")
			}
			gitProvider, err := models.GitProviderFromString(provider)
			if err != nil {
				return result, errors.New("bad git origin, illegal provider")
			}
			result.Git.Provider = gitProvider
		}

		// For git check for the optional branch as well.
		branch, found, err := unstructured.NestedString(origin, "git", "branch")
		if found {
			if err != nil {
				return result, err
			}
			if branch == "" {
				return result, errors.New("bad git origin, branch is empty string")
			}
			result.Git.Branch = branch
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
