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

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// List returns a slice of all known builderimage CRs.
func List(
	ctx context.Context,
	cluster *kubernetes.Cluster,
) (models.BuilderImageList, error) {
	client, err := cluster.ClientBuilderImage()
	if err != nil {
		return nil, err
	}

	list, err := client.Namespace(helmchart.Namespace()).List(
		ctx,
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, err
	}

	builderimages := make(models.BuilderImageList, 0, len(list.Items))

	for _, bp := range list.Items {
		copy := bp // Prevent memory aliasing warning
		builderimage, err := toBuilderImage(&copy)
		if err != nil {
			return nil, err
		}
		builderimages = append(builderimages, *builderimage)
	}

	return builderimages, nil
}

// Exists tests if the named builderimage exists, or not.
func Exists(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
) (bool, error) {
	_, err := Get(ctx, cluster, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Lookup returns the named builderimage, or nil if it does not exist.
func Lookup(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
) (*models.BuilderImage, error) {
	bpCR, err := Get(ctx, cluster, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return toBuilderImage(bpCR)
}

// Get returns the builderimage resource from the cluster.
func Get(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
) (*unstructured.Unstructured, error) {
	client, err := cluster.ClientBuilderImage()
	if err != nil {
		return nil, err
	}

	return client.Namespace(helmchart.Namespace()).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
}

// toBuilderImage converts the unstructured builderimage CR into the public DTO.
func toBuilderImage(bp *unstructured.Unstructured) (*models.BuilderImage, error) {
	name, _, err := unstructured.NestedString(
		bp.UnstructuredContent(),
		"metadata",
		"name",
	)
	if err != nil {
		return nil, errors.New("builderimage name should be string")
	}

	image, _, err := unstructured.NestedString(
		bp.UnstructuredContent(),
		"spec",
		"image",
	)
	if err != nil {
		return nil, errors.New("image should be string")
	}

	description, _, err := unstructured.NestedString(
		bp.UnstructuredContent(),
		"spec",
		"description",
	)
	if err != nil {
		return nil, errors.New("description should be string")
	}

	short, _, err := unstructured.NestedString(
		bp.UnstructuredContent(),
		"spec",
		"shortDescription",
	)
	if err != nil {
		return nil, errors.New("shortDescription should be string")
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
	}, nil
}
