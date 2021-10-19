// Package response is used by all actions to write their final result as JSON
package response

import (
	"net/http"

	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// OK reports a generic success
func OK(c *gin.Context) {
	c.JSON(http.StatusOK, models.ResponseOK)
}

// OKReturn reports a success with some data
func OKReturn(c *gin.Context, response interface{}) {
	c.JSON(http.StatusOK, response)
}

// Created reports successful creation of a resource.
func Created(c *gin.Context) {
	c.JSON(http.StatusCreated, models.ResponseOK)
}

// Error reports the specified errors
func Error(c *gin.Context, responseErrors errors.APIErrors) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.JSON(responseErrors.FirstStatus(), errors.ErrorResponse{
		Errors: responseErrors.Errors(),
	})
}
