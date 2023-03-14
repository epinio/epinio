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

package docs

import "github.com/epinio/epinio/pkg/api/core/v1/models"

//go:generate swagger generate spec

// swagger:route GET /namespaces namespace Namespaces
// Return list of all controlled namespaces.
// responses:
//   200: NamespacesResponse

// swagger:response NamespacesResponse
type NamespacesResponse struct {
	// in: body
	Body models.NamespaceList
}

// swagger:route POST /namespaces namespace NamespaceCreate
// Create the posted new namespace.
// responses:
//   200: NamespaceCreateResponse

// swagger:parameters NamespaceCreate
type NamespaceCreateParam struct {
	// in: body
	Body models.NamespaceCreateRequest
}

// swagger:response NamespaceCreateResponse
type NamespaceCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route DELETE /namespaces NamespaceBatchDelete
// Delete the named namespaces.
// responses:
//   200: NamespaceDeleteResponse

// swagger:route DELETE /namespaces/{Namespace} namespace NamespaceDelete
// Delete the named `Namespace`.
// responses:
//   200: NamespaceDeleteResponse

// swagger:parameters NamespaceDelete
type NamespaceDeleteParam struct {
	// in: path
	Namespace string
}

// swagger:parameters NamespaceBatchDelete
type NamespaceBatchDeleteParam struct {
	// in: url
	Namespaces []string
}

// swagger:response NamespaceDeleteResponse
type NamespaceDeleteResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{Namespace} namespace NamespaceShow
// Return details of the named `Namespace`.
// responses:
//   200: NamespaceShowResponse

// swagger:parameters NamespaceShow
type NamespaceShowParam struct {
	// in: path
	Namespace string
}

// swagger:response NamespaceShowResponse
type NamespaceShowResponse struct {
	// in: body
	Body models.Namespace
}

// swagger:route GET /namespacematches/{Pattern} namespace NamespaceMatch
// Return list of names for all controlled namespaces whose name matches the prefix `Pattern`.
// responses:
//   200: NamespaceMatchResponse

// swagger:parameters NamespaceMatch
type NamespaceMatchParam struct {
	// in: path
	Pattern string
}

// swagger:response NamespaceMatchResponse
type NamespaceMatchResponse struct {
	// in: body
	Body models.NamespacesMatchResponse
}

// swagger:route GET /namespacematches namespace NamespaceMatch0
// Return list of names for all controlled namespaces (No prefix = empty prefix = match everything)
// responses:
//   200: NamespaceMatchResponse

// swagger:parameters NamespaceMatch0
type NamespaceMatch0Param struct{}

// response: See NamespaceMatch.
