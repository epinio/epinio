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

package errors

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorResponse is the response's JSON, that is send in case of an error
type ErrorResponse struct {
	Errors []APIError `json:"errors"`
}

// APIErrors is the interface used by all handlers to return one or more errors
type APIErrors interface {
	Errors() []APIError
	FirstStatus() int
}

// APIError fulfills the error and APIErrors interfaces. It contains a single error.
type APIError struct {
	Status  int    `json:"status"`
	Title   string `json:"title"`
	Details string `json:"details,omitempty"`
}

var _ APIErrors = APIError{}
var _ error = APIError{}

// Error satisfies the error interface
func (a APIError) Error() string {
	return a.Title
}

// Errors satisfies the APIErrors interface
func (a APIError) Errors() []APIError {
	return []APIError{a}
}

// FirstStatus (APIErrors interface) returns the stored error's status
func (a APIError) FirstStatus() int {
	return a.Status
}

// NewAPIError constructs an APIerror from basics
func NewAPIError(title string, status int) APIError {
	return APIError{
		Title:  title,
		Status: status,
	}
}

// WithDetails returns a new error with the provided details
func (a APIError) WithDetails(details string) APIError {
	a.Details = details
	return a
}

// WithDetailsf returns a new error with the provided details formatted as specified
func (a APIError) WithDetailsf(format string, values ...any) APIError {
	a.Details = fmt.Sprintf(format, values...)
	return a
}

// MultiError fulfills the APIErrors interface. It contains multiple errors.
type MultiError struct {
	errors []APIError
}

var _ APIErrors = MultiError{}
var _ error = MultiError{}

// Error satisfies the error interface
func (m MultiError) Error() string {
	return m.errors[0].Title
}

// Errors satisfies the APIErrors interface
func (m MultiError) Errors() []APIError {
	return m.errors
}

// FirstStatus (APIErrors interface) returns the status of the first error stored
func (m MultiError) FirstStatus() int {
	return m.errors[0].Status
}

// NewMultiError constructs an APIerror from basics
func NewMultiError(errs []APIError) MultiError {
	return MultiError{errors: errs}
}

// InternalError constructs an API error for server internal issues, from a lower-level error
func InternalError(err error, details ...string) APIError {
	joinedDetails := strings.Join(details, ", ")
	return NewAPIError(
		err.Error(),
		http.StatusInternalServerError,
	).WithDetailsf("%s\nServer Backtrace: %+v", joinedDetails, err)
}

// NewInternalError constructs an API error for server internal issues, from a message
func NewInternalError(msg string, details ...string) APIError {
	return NewAPIError(msg, http.StatusInternalServerError).WithDetails(strings.Join(details, ", "))
}

// NewBadRequestError constructs an API error for general issues with a request, from a message
func NewBadRequestError(msg string) APIError {
	return NewAPIError(msg, http.StatusBadRequest)
}

// NewBadRequestErrorf constructs an API error for general issues with a request, with a formatted message
func NewBadRequestErrorf(format string, values ...any) APIError {
	return NewAPIError(fmt.Sprintf(format, values...), http.StatusBadRequest)
}

// NewNotFoundError constructs a general API error for when something desired does not exist
func NewNotFoundError(kind, name string) APIError {
	msg := fmt.Sprintf("%s '%s' does not exist", kind, name)
	return NewAPIError(msg, http.StatusNotFound)
}

// NewConflictError constructs a general API error for when something conflicts
func NewConflictError(kind, name string) APIError {
	msg := fmt.Sprintf("%s '%s' already exists", kind, name)
	return NewAPIError(msg, http.StatusConflict)
}

////////////////////////////////////////////
//
// Helpers to build common "domain" errors
//
////////////////////////////////////////////

/////////////////////////
//
// NotFound (404) errors
//
/////////////////////////

// NamespaceIsNotKnown constructs an API error for when the desired namespace does not exist
func NamespaceIsNotKnown(namespace string) APIError {
	return NewNotFoundError("namespace", namespace)
}

// AppIsNotKnown constructs an API error for when the desired app does not exist
func AppIsNotKnown(app string) APIError {
	return NewNotFoundError("application", app)
}

// ServiceIsNotKnown constructs an API error for when the desired service does not exist
func ServiceIsNotKnown(service string) APIError {
	return NewNotFoundError("service", service)
}

// ConfigurationIsNotKnown constructs an API error for when the desired configuration instance does not exist
func ConfigurationIsNotKnown(configuration string) APIError {
	return NewNotFoundError("configuration", configuration)
}

// AppChartIsNotKnown constructs an API error for when the desired app chart does not exist
func AppChartIsNotKnown(appChart string) APIError {
	return NewNotFoundError("application chart", appChart)
}

/////////////////////////
//
// Conflict (409) errors
//
/////////////////////////

// AppAlreadyKnown constructs an API error for when we have a conflict with an existing app
func AppAlreadyKnown(app string) APIError {
	return NewConflictError("application", app)
}

// NamespaceAlreadyKnown constructs an API error for when we have a conflict with an existing namespace
func NamespaceAlreadyKnown(namespace string) APIError {
	return NewConflictError("namespace", namespace)
}

// ConfigurationAlreadyKnown constructs an API error for when we have a conflict with an existing configuration instance
func ConfigurationAlreadyKnown(configuration string) APIError {
	return NewConflictError("configuration", configuration)
}

// ServiceAlreadyKnown constructs an API error for when we have a conflict with an existing service instance
func ServiceAlreadyKnown(service string) APIError {
	return NewConflictError("service", service)
}
