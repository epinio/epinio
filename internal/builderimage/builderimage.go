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

// Package builderimage collects the structures and functions that deal with
// epinio's BuilderImage CR (application.epinio.io/v1, kind BuilderImage).
package builderimage

import (
	"context"
	"errors"

	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// List returns a slice of all known builderimage CRs.
func List(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
) (models.BuilderImageList, error) {
	list, listError := client.Namespace(helmchart.Namespace()).List(
		ctx,
		metav1.ListOptions{},
	)
	if listError != nil {
		return nil, listError
	}

	builderimages := make(models.BuilderImageList, 0, len(list.Items))

	for _, item := range list.Items {
		copyItem := item // Prevent memory aliasing warning
		converted, convertError := toBuilderImage(&copyItem)
		if convertError != nil {
			return nil, convertError
		}
		builderimages = append(builderimages, *converted)
	}

	return builderimages, nil
}

// Exists tests if the named builderimage exists, or not.
func Exists(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	name string,
) (bool, error) {
	_, getError := Get(ctx, client, name)
	if getError != nil {
		if apierrors.IsNotFound(getError) {
			return false, nil
		}
		return false, getError
	}
	return true, nil
}

// Lookup returns the named builderimage, or nil if it does not exist.
func Lookup(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	name string,
) (*models.BuilderImage, error) {
	bpCR, getError := Get(ctx, client, name)
	if getError != nil {
		if apierrors.IsNotFound(getError) {
			return nil, nil
		}
		return nil, getError
	}

	return toBuilderImage(bpCR)
}

// Get returns the builderimage resource from the cluster.
func Get(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	name string,
) (*unstructured.Unstructured, error) {
	return client.Namespace(helmchart.Namespace()).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
}

// toBuilderImage converts the unstructured builderimage CR into the public DTO.
func toBuilderImage(bp *unstructured.Unstructured) (*models.BuilderImage, error) {
	name, _, nameError := unstructured.NestedString(
		bp.UnstructuredContent(),
		"metadata",
		"name",
	)
	if nameError != nil {
		return nil, errors.New("builderimage name should be string")
	}

	image, _, imageError := unstructured.NestedString(
		bp.UnstructuredContent(),
		"spec",
		"image",
	)
	if imageError != nil {
		return nil, errors.New("image should be string")
	}

	description, _, descriptionError := unstructured.NestedString(
		bp.UnstructuredContent(),
		"spec",
		"description",
	)
	if descriptionError != nil {
		return nil, errors.New("description should be string")
	}

	short, _, shortError := unstructured.NestedString(
		bp.UnstructuredContent(),
		"spec",
		"shortDescription",
	)
	if shortError != nil {
		return nil, errors.New("shortDescription should be string")
	}

	// default is operator policy (helm seed / kubectl), surfaced read-only.
	// A missing field reads as false.
	isDefault, _, defaultError := unstructured.NestedBool(
		bp.UnstructuredContent(),
		"spec",
		"default",
	)
	if defaultError != nil {
		return nil, errors.New("default should be bool")
	}

	createdAt := bp.GetCreationTimestamp()

	return &models.BuilderImage{
		Meta: models.MetaLite{
			Name:      name,
			CreatedAt: createdAt,
		},
		Image:            image,
		Description:      description,
		ShortDescription: short,
		Default:          isDefault,
	}, nil
}
