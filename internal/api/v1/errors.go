package v1

import (
	"fmt"
	"net/http"
	"strings"
)

// APIActionFunc is matched by all actions. Actions can return a list of errors.
// The "Status" of the first error in the list becomes the response Status Code.
type APIActionFunc func(http.ResponseWriter, *http.Request) APIErrors

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
	Details string `json:"details"`
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
func NewAPIError(title string, details string, status int) APIError {
	return APIError{
		Title:   title,
		Details: details,
		Status:  status,
	}
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

// InternalError constructs an API error for server internal issues, from a lower-level error
func InternalError(err error, details ...string) APIError {
	return NewAPIError(err.Error(), strings.Join(details, ", "), http.StatusInternalServerError)
}

// InternalError constructs an API error for server internal issues, from a message
func NewInternalError(msg string, details ...string) APIError {
	return NewAPIError(msg, strings.Join(details, ", "), http.StatusInternalServerError)
}

// BadRequest constructs an API error for general issues with a request, from a lower-level error
func BadRequest(err error, details ...string) APIError {
	return NewAPIError(err.Error(), strings.Join(details, ", "), http.StatusBadRequest)
}

// NewBadRequest constructs an API error for general issues with a request, from a message
func NewBadRequest(msg string, details ...string) APIError {
	return NewAPIError(msg, strings.Join(details, ", "), http.StatusBadRequest)
}

// NewNotFoundError constructs a general API error for when something desired does not exist
func NewNotFoundError(msg string, details ...string) APIError {
	return NewAPIError(msg, strings.Join(details, ", "), http.StatusNotFound)
}

// OrgIsNotKnown constructs an API error for when the desired org does not exist
func OrgIsNotKnown(org string) APIError {
	return NewAPIError(
		fmt.Sprintf("Organization '%s' does not exist", org),
		"",
		http.StatusNotFound)
}

// AppAlreadyKnown constructs an API error for when we have a conflict with an existing app
func AppAlreadyKnown(app string) APIError {
	return NewAPIError(
		fmt.Sprintf("Application '%s' already exists", app),
		"",
		http.StatusConflict)
}

// AppIsNotKnown constructs an API error for when the desired app does not exist
func AppIsNotKnown(app string) APIError {
	return NewAPIError(
		fmt.Sprintf("Application '%s' does not exist", app),
		"",
		http.StatusNotFound)
}

// ServiceIsNotKnown constructs an API error for when the desired service instance does not exist
func ServiceIsNotKnown(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' does not exist", service),
		"",
		http.StatusNotFound)
}

// ServiceClassIsNotKnown constructs an API error for when the desired service class does not exist
func ServiceClassIsNotKnown(serviceclass string) APIError {
	return NewAPIError(
		fmt.Sprintf("ServiceClass '%s' does not exist", serviceclass),
		"",
		http.StatusNotFound)
}

// ServicePlanIsNotKnown constructs an API error for when the desired service plan does not exist
func ServicePlanIsNotKnown(service string, c string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service plan '%s' does not exist for class '%s'", service, c),
		"",
		http.StatusNotFound)
}

// OrgAlreadyKnown constructs an API error for when we have a conflict with an existing org
func OrgAlreadyKnown(org string) APIError {
	return NewAPIError(
		fmt.Sprintf("Organization '%s' already exists", org),
		"",
		http.StatusConflict)
}

// ServiceAlreadyKnown constructs an API error for when we have a conflict with an existing service instance
func ServiceAlreadyKnown(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' already exists", service),
		"",
		http.StatusConflict)
}

// ServiceAlreadyBound constructs an API error for when the service to bind is already bound to the app
func ServiceAlreadyBound(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' already bound", service),
		"",
		http.StatusConflict)
}

// ServiceIsNotBound constructs an API error for when the service to unbind is actually not bound to the app
func ServiceIsNotBound(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' is not bound", service),
		"",
		http.StatusBadRequest)
}
