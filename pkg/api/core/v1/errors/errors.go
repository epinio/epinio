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
func NewBadRequestErrorf(format string, values ...string) APIError {
	return NewAPIError(fmt.Sprintf(format, values), http.StatusBadRequest)
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

// UserNotFound constructs an API error for when the user name is not found in the header
func UserNotFound() APIError {
	return NewBadRequestError("user not found in the request header")
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

// AppChartAlreadyKnown constructs an API error for when we have a conflict with an existing app chart
func AppChartAlreadyKnown(appChart string) APIError {
	return NewConflictError("application chart", appChart)
}

// ConfigurationAlreadyBound constructs an API error for when the configuration to bind is already bound to the app
func ConfigurationAlreadyBound(configuration string) APIError {
	msg := fmt.Sprintf("configuration '%s' already bound", configuration)
	return NewAPIError(msg, http.StatusConflict)
}

///////////////////////////
//
// BadRequest (400) errors
//
///////////////////////////

// ConfigurationIsNotBound constructs an API error for when the configuration to unbind is actually not bound to the app
func ConfigurationIsNotBound(configuration string) APIError {
	return NewBadRequestErrorf("configuration '%s' is not bound", configuration)
}
