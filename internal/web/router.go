// Package web implements the Epinio dashboard
package web

import (
	"github.com/gin-gonic/gin"
)

// Lemon extends the specified router with the methods and urls
// handling the dashboard endpoints
func Lemon(router *gin.Engine) {
	router.GET("/", gin.Logger(), ApplicationsController{}.Index)
	router.GET("/info", gin.Logger(), InfoController{}.Index)
	router.GET("/orgs/target/:org", gin.Logger(), OrgsController{}.Target)
}
