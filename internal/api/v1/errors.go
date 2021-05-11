package v1

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorResponse is the response's JSON, that is send in case of an error
type ErrorResponse struct {
	Errors APIErrors `json:"errors"`
}

type APIError struct {
	Status  int    `json:"status"`
	Title   string `json:"title"`
	Details string `json:"details"`
}

// Satisfy the error interface
func (err APIError) Error() string {
	return err.Title
}

func NewAPIError(message, details string, status int) APIError {
	return APIError{
		Title:   message,
		Details: details,
		Status:  status,
	}
}

type APIErrors []APIError

// NewAPIErrors returns a list of APIError
func NewAPIErrors(errs ...APIError) APIErrors {
	return errs
}

// singleNewError helps to return just a single error, as a list
func singleNewError(message string, status int) APIErrors {
	return NewAPIErrors(NewAPIError(message, "", status))
}

// singleError helps to return just a single error, as a list
func singleError(err error, status int) APIErrors {
	return NewAPIErrors(NewAPIError(err.Error(), "", status))
}

// singleInternalError is a helper to return a single 5xx error, with a message, in a list.
func singleInternalError(err error, msg string) APIErrors {
	return NewAPIErrors(NewAPIError(err.Error(), msg, http.StatusInternalServerError))
}

// All our actions match this type. They can return a list of errors.
// The "Status" of the first error in the list becomes the response Status Code.
type APIActionFunc func(http.ResponseWriter, *http.Request) APIErrors

func InternalError(err error) APIError {
	return NewAPIError(err.Error(), "", http.StatusInternalServerError)
}

func BadRequest(err error, details ...string) APIError {
	return NewAPIError(err.Error(), strings.Join(details, ", "), http.StatusBadRequest)
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
