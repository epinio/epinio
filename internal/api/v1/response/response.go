// Package response is used by all actions to write their final result as JSON
package response

import (
	"net/http"

	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// JSONWithStatus writes the response struct as JSON to the writer
func JSONWithStatus(c *gin.Context, code int, response interface{}) error {
	c.JSON(code, response)
	return nil
}

// JSON writes the response struct as JSON to the writer
func JSON(c *gin.Context, response interface{}) error {
	c.JSON(http.StatusOK, response)
	return nil
}

// JSONError writes the error as a JSON response to the writer
func JSONError(c *gin.Context, responseErrors errors.APIErrors) {
	response := errors.ErrorResponse{
		Errors: responseErrors.Errors(),
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(responseErrors.FirstStatus(), response)
}
