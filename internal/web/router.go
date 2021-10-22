// Package web implements the Epinio dashboard
package web

import (
	"github.com/gin-gonic/gin"
)

// Lemon extends the specified router with the methods and urls
// handling the dashboard endpoints
func Lemon(router *gin.RouterGroup) {
	router.GET("/", ApplicationsController{}.Index)
	router.GET("/info", InfoController{}.Index)
	router.GET("/orgs/target/:org", OrgsController{}.Target)
}
