// Package routes implements registered urls and parameter substitution
package routes

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// Route describes a route for httprouter
type Route struct {
	Name    string
	Method  string
	Path    string
	Format  string
	Handler gin.HandlerFunc
}

var formatRegex = regexp.MustCompile(`:\w+`)

// NewRoute returns a new route, which can be added to NamedRoutes and
// used with gin. Trailing and leading slashes are removed.
func NewRoute(name, method, path string, h gin.HandlerFunc) Route {
	format := formatRegex.ReplaceAllString(path, "%s")
	format = strings.Trim(format, "/")
	return Route{name, method, path, format, h}
}

// NamedRoutes is a map of all named routes, to provide something like
// https://github.com/gorilla/mux#registered-urls
type NamedRoutes map[string]Route

// Path returns a route's path with params substituted, panics if
// used inproperly.
func (n NamedRoutes) Path(name string, params ...interface{}) string {
	r, ok := n[name]
	if !ok {
		// this means you have a typo or you allowed a user controlled
		// string to reach this method, don't.
		panic(fmt.Sprintf("route not found for '%s'", name))
	}

	// make sure to pass the right amount of params
	// otherwise this method might fail
	if r.Format == "" || len(params) == 0 {
		return strings.Trim(r.Path, "/")
	}
	return fmt.Sprintf(r.Format, params...)
}

func (n NamedRoutes) SetRoutes(routes ...Route) {
	// TODO add some validation to prevent overriding and such
	for _, route := range routes {
		n[route.Name] = route
	}
}
