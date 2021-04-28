// Package routes implements registered urls and parameter substitution
package routes

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// Route describes a route for httprouter
type Route struct {
	Method  string
	Path    string
	Format  string
	Handler http.HandlerFunc
}

var formatRegex = regexp.MustCompile(`:\w+`)

// NewRoute returns a new route, which can be added to NamedRoutes and used with
// httprouter. Trailing and leading slashes are removed.
func NewRoute(method string, path string, h http.HandlerFunc) Route {
	format := formatRegex.ReplaceAllString(path, "%s")
	format = strings.Trim(format, "/")
	return Route{method, path, format, h}
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
