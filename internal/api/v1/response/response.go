// Package response is used by all actions to write their final result as JSON
package response

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// JSON writes the response struct as JSON to the writer
func JSON(w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = w.Write(js)
	return err
}

// JSONError writes the error as a JSON response to the writer
func JSONError(w http.ResponseWriter, responseErrors errors.APIErrors) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	response := errors.ErrorResponse{Errors: responseErrors.Errors()}
	js, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, marshalErr.Error())
		return
	}

	w.WriteHeader(responseErrors.FirstStatus())
	fmt.Fprintln(w, string(js))
}
