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

package application_test

import (
	"context"
	"errors"

	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/testfakes/k8sdynamic/k8sdynamicfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("BuilderImagesInUse", func() {
	var (
		ctx    context.Context
		fakeRI *k8sdynamicfakes.FakeResourceInterface
		fakeNS *k8sdynamicfakes.FakeNamespaceableResourceInterface
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeRI = &k8sdynamicfakes.FakeResourceInterface{}
		fakeNS = &k8sdynamicfakes.FakeNamespaceableResourceInterface{}
		fakeNS.NamespaceReturns(fakeRI)
	})

	// appCR builds an app resource carrying the staged builder image string the
	// way the staging endpoint persists it under spec.builderimage.
	appCR := func(name, builderImage string) unstructured.Unstructured {
		spec := map[string]interface{}{}
		if builderImage != "" {
			spec["builderimage"] = builderImage
		}
		return unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": name},
				"spec":     spec,
			},
		}
	}

	It("keys the set on the app's staged builder image string, not the CR name", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				appCR("app1", "paketobuildpacks/builder-jammy-full:0.3.495"),
				appCR("app2", "registry.example.com/custom:latest"),
			},
		}, nil)

		inUse, err := application.BuilderImagesInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(2))
		Expect(inUse).To(HaveKeyWithValue("paketobuildpacks/builder-jammy-full:0.3.495", true))
		Expect(inUse).To(HaveKeyWithValue("registry.example.com/custom:latest", true))
	})

	It("skips apps with no builder image (e.g. image-based deploys)", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				appCR("app1", ""),
				appCR("app2", "paketo:jammy"),
			},
		}, nil)

		inUse, err := application.BuilderImagesInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(1))
		Expect(inUse).To(HaveKey("paketo:jammy"))
	})

	It("collapses multiple apps on the same builder image to one key", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				appCR("app1", "paketo:jammy"),
				appCR("app2", "paketo:jammy"),
			},
		}, nil)

		inUse, err := application.BuilderImagesInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(1))
	})

	It("returns an empty set when no apps exist", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{}, nil)

		inUse, err := application.BuilderImagesInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(BeEmpty())
	})

	It("propagates list errors", func() {
		fakeRI.ListReturns(nil, errors.New("boom"))

		_, err := application.BuilderImagesInUse(ctx, fakeNS)
		Expect(err).To(HaveOccurred())
	})
})
