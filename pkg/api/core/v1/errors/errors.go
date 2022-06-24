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

// NewMultiError constructs an APIerror from basics
func NewMultiError(errs []APIError) MultiError {
	return MultiError{errors: errs}
}

// InternalError constructs an API error for server internal issues, from a lower-level error
func InternalError(err error, details ...string) APIError {
	return NewAPIError(
		err.Error(),
		strings.Join(details, ", ")+fmt.Sprintf("\nServer Backtrace: %+v", err),
		http.StatusInternalServerError,
	)
}

// NewInternalError constructs an API error for server internal issues, from a message
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
func NewNotFoundError(err error, details ...string) APIError {
	return NewAPIError(err.Error(), strings.Join(details, ", "), http.StatusNotFound)
}

// NewConflictError constructs a general API error for when something conflicts
func NewConflictError(err error, details ...string) APIError {
	return NewAPIError(err.Error(), strings.Join(details, ", "), http.StatusConflict)
}

////////////////////////////////////////////
//
// Helpers to build common "domain" errors
//
////////////////////////////////////////////

// NamespaceIsNotKnown constructs an API error for when the desired namespace does not exist
func NamespaceIsNotKnown(namespace string) APIError {
	return NewNotFoundError(fmt.Errorf("Targeted namespace '%s' does not exist", namespace))
}

// UserNotFound constructs an API error for when the user name is not found in the header
func UserNotFound() APIError {
	return NewAPIError(
		"User not found in the request header",
		"",
		http.StatusBadRequest)
}

// AppAlreadyKnown constructs an API error for when we have a conflict with an existing app
func AppAlreadyKnown(app string) APIError {
	return NewConflictError(fmt.Errorf("Application '%s' already exists", app))
}

// AppIsNotKnown constructs an API error for when the desired app does not exist
func AppIsNotKnown(app string) APIError {
	return NewNotFoundError(fmt.Errorf("Application '%s' does not exist", app))
}

// ServiceIsNotKnown constructs an API error for when the desired service does not exist
func ServiceIsNotKnown(service string) APIError {
	return NewNotFoundError(fmt.Errorf("Service '%s' does not exist", service))
}

// ConfigurationIsNotKnown constructs an API error for when the desired configuration instance does not exist
func ConfigurationIsNotKnown(configuration string) APIError {
	return NewNotFoundError(fmt.Errorf("Configuration '%s' does not exist", configuration))
}

// NamespaceAlreadyKnown constructs an API error for when we have a conflict with an existing namespace
func NamespaceAlreadyKnown(namespace string) APIError {
	return NewConflictError(fmt.Errorf("Namespace '%s' already exists", namespace))
}

// ConfigurationAlreadyKnown constructs an API error for when we have a conflict with an existing configuration instance
func ConfigurationAlreadyKnown(configuration string) APIError {
	return NewConflictError(fmt.Errorf("Configuration '%s' already exists", configuration))
}

// ConfigurationAlreadyBound constructs an API error for when the configuration to bind is already bound to the app
func ConfigurationAlreadyBound(configuration string) APIError {
	return NewConflictError(fmt.Errorf("Configuration '%s' already bound", configuration))
}

// ConfigurationIsNotBound constructs an API error for when the configuration to unbind is actually not bound to the app
func ConfigurationIsNotBound(configuration string) APIError {
	return NewAPIError(
		fmt.Sprintf("Configuration '%s' is not bound", configuration),
		"",
		http.StatusBadRequest)
}

// AppChartAlreadyKnown constructs an API error for when we have a conflict with an existing app chart
func AppChartAlreadyKnown(app string) APIError {
	return NewConflictError(fmt.Errorf("Application Chart '%s' already exists", app))
}

// AppChartIsNotKnown constructs an API error for when the desired app chart does not exist
func AppChartIsNotKnown(app string) APIError {
	return NewNotFoundError(fmt.Errorf("Application Chart '%s' does not exist", app))
}
