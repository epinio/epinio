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

var _ = Describe("GitconfigsInUse", func() {
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

	// gitAppCR builds an app resource with a git origin, carrying the selected
	// git configuration the way push persists it under
	// spec.origin.git.gitconfig.
	gitAppCR := func(name, gitconfig string) unstructured.Unstructured {
		git := map[string]interface{}{
			"repository": "https://github.com/example/repo",
		}
		if gitconfig != "" {
			git["gitconfig"] = gitconfig
		}

		return unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": name},
				"spec": map[string]interface{}{
					"origin": map[string]interface{}{"git": git},
				},
			},
		}
	}

	// pathAppCR builds an app pushed from a local folder: no git origin at all.
	pathAppCR := func(name string) unstructured.Unstructured {
		return unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": name},
				"spec": map[string]interface{}{
					"origin": map[string]interface{}{"path": "blob-uid"},
				},
			},
		}
	}

	It("keys the set on the app's selected git configuration", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				gitAppCR("app1", "github-corp"),
				gitAppCR("app2", "gitlab-selfhosted"),
			},
		}, nil)

		inUse, err := application.GitconfigsInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(2))
		Expect(inUse).To(HaveKeyWithValue("github-corp", true))
		Expect(inUse).To(HaveKeyWithValue("gitlab-selfhosted", true))
	})

	It("skips git apps with no gitconfig (public repo, or pre-feature)", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				gitAppCR("app1", ""),
				gitAppCR("app2", "github-corp"),
			},
		}, nil)

		inUse, err := application.GitconfigsInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(1))
		Expect(inUse).To(HaveKey("github-corp"))
	})

	It("skips apps whose origin is not git", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				pathAppCR("app1"),
				gitAppCR("app2", "github-corp"),
			},
		}, nil)

		inUse, err := application.GitconfigsInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(1))
		Expect(inUse).To(HaveKey("github-corp"))
	})

	It("collapses multiple apps on the same gitconfig to one key", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				gitAppCR("app1", "github-corp"),
				gitAppCR("app2", "github-corp"),
			},
		}, nil)

		inUse, err := application.GitconfigsInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(HaveLen(1))
	})

	It("returns an empty set when no apps exist", func() {
		fakeRI.ListReturns(&unstructured.UnstructuredList{}, nil)

		inUse, err := application.GitconfigsInUse(ctx, fakeNS)
		Expect(err).ToNot(HaveOccurred())
		Expect(inUse).To(BeEmpty())
	})

	It("propagates list errors", func() {
		fakeRI.ListReturns(nil, errors.New("boom"))

		_, err := application.GitconfigsInUse(ctx, fakeNS)
		Expect(err).To(HaveOccurred())
	})
})
