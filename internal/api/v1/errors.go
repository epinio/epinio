package v1

import (
	"fmt"
	"net/http"
	"strings"
)

type APIError struct {
	Status  int
	Title   string
	Details string
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
