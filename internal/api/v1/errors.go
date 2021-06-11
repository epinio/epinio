package v1

import (
	"fmt"
	"net/http"
	"strings"
)

// All our actions match this type. They can return a list of errors.
// The "Status" of the first error in the list becomes the response Status Code.
type APIActionFunc func(http.ResponseWriter, *http.Request) APIErrors

// ErrorResponse is the response's JSON, that is send in case of an error
type ErrorResponse struct {
	Errors []APIError `json:"errors"`
}

type APIErrors interface {
	Errors() []APIError
	FirstStatus() int
}

type APIError struct {
	Status  int    `json:"status"`
	Title   string `json:"title"`
	Details string `json:"details"`
}

var _ APIErrors = APIError{}

// Satisfy the error interface
func (err APIError) Error() string {
	return err.Title
}

// Satisfy the multi error interface
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

var _ APIErrors = MultiError{}

type MultiError struct {
	errors []APIError
}

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
