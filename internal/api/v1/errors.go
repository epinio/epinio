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

// APIErrors interface is used by all the handlers to return a single or
// multiple errors
type APIErrors interface {
	Errors() []APIError
	FirstStatus() int
}

// APIError fulfills the error and the APIErrors interface and contains a single error
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

func (a APIError) FirstStatus() int {
	return a.Status
}

func NewAPIError(title string, details string, status int) APIError {
	return APIError{
		Title:   title,
		Details: details,
		Status:  status,
	}
}

// MultiError fulfills the APIErrors interface and contains multiple errors
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

func (m MultiError) FirstStatus() int {
	return m.errors[0].Status
}

func InternalError(err error, details ...string) APIError {
	return NewAPIError(err.Error(), strings.Join(details, ", "), http.StatusInternalServerError)
}

func NewInternalError(msg string, details ...string) APIError {
	return NewAPIError(msg, strings.Join(details, ", "), http.StatusInternalServerError)
}

func BadRequest(err error, details ...string) APIError {
	return NewAPIError(err.Error(), strings.Join(details, ", "), http.StatusBadRequest)
}

func NewBadRequest(msg string, details ...string) APIError {
	return NewAPIError(msg, strings.Join(details, ", "), http.StatusBadRequest)
}

func NewNotFoundError(msg string, details ...string) APIError {
	return NewAPIError(msg, strings.Join(details, ", "), http.StatusNotFound)
}

func OrgIsNotKnown(org string) APIError {
	return NewAPIError(
		fmt.Sprintf("Organization '%s' does not exist", org),
		"",
		http.StatusNotFound)
}

func AppAlreadyKnown(app string) APIError {
	return NewAPIError(
		fmt.Sprintf("Application '%s' already exists", app),
		"",
		http.StatusConflict)
}

func AppIsNotKnown(app string) APIError {
	return NewAPIError(
		fmt.Sprintf("Application '%s' does not exist", app),
		"",
		http.StatusNotFound)
}

func ServiceIsNotKnown(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' does not exist", service),
		"",
		http.StatusNotFound)
}

func ServiceClassIsNotKnown(serviceclass string) APIError {
	return NewAPIError(
		fmt.Sprintf("ServiceClass '%s' does not exist", serviceclass),
		"",
		http.StatusNotFound)
}

func ServicePlanIsNotKnown(service string, c string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service plan '%s' does not exist for class '%s'", service, c),
		"",
		http.StatusNotFound)
}

func OrgAlreadyKnown(org string) APIError {
	return NewAPIError(
		fmt.Sprintf("Organization '%s' already exists", org),
		"",
		http.StatusConflict)
}

func ServiceAlreadyKnown(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' already exists", service),
		"",
		http.StatusConflict)
}

func ServiceAlreadyBound(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' already bound", service),
		"",
		http.StatusConflict)
}

func ServiceIsNotBound(service string) APIError {
	return NewAPIError(
		fmt.Sprintf("Service '%s' is not bound", service),
		"",
		http.StatusBadRequest)
}
