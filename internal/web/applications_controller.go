package web

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/filesystem"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/gin-gonic/gin"
)

// ApplicationsController represents all general functionality of the dashboard
type ApplicationsController struct {
}

// setCurrentNamespaceInCookie is a helper for creating cookies to persist system state in the browser
func setCurrentNamespaceInCookie(namespace, cookieName string, c *gin.Context) {
	c.SetCookie(cookieName,
		namespace,
		365*24*60*60, // 1 year
		"/",
		"",
		false,
		false,
	)
}

// getNamespaces tries to decide what the current namespace is.
// First looks in the cookie named "currentNamespace". If it is exists and the namespace
// set there still exists that's our current namespace. If the cookie exists
// but the namespace does not (e.g. because it was deleted) or if the cookie does not
// exist, then the first existing namespace becomes the current namespace and the cookie is
// updated. If no namespaces exist, then an empty string is returned as the namespace name.
// The function also returns the rest of the available namespaces.
func getNamespaces(c *gin.Context) (string, []string, error) {

	ctx := c.Request.Context()
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return "", []string{}, err
	}

	allNamespaces, err := namespaces.List(ctx, cluster)
	if err != nil {
		return "", []string{}, err
	}
	if len(allNamespaces) == 0 {
		return "", []string{}, nil
	}

	otherNamespaces := func(current string, allNamespaces []namespaces.Namespace) []string {
		other := []string{}
		for _, namespace := range allNamespaces {
			if namespace.Name != current {
				other = append(other, namespace.Name)
			}
		}
		return other
	}

	cookie, err := c.Request.Cookie("currentNamespace")
	if err != nil {
		// There was no cookie, let's create one
		if err == http.ErrNoCookie {
			currentNamespace := allNamespaces[0].Name
			rest := otherNamespaces(currentNamespace, allNamespaces)
			setCurrentNamespaceInCookie(currentNamespace, "currentNamespace", c)
			return currentNamespace, rest, nil
		}
		return "", []string{}, err
	}
	namespaceExists := func(cookieNamespace string, allNamespaces []namespaces.Namespace) bool {
		for _, namespace := range allNamespaces {
			if namespace.Name == cookieNamespace {
				return true
			}
		}
		return false
	}(cookie.Value, allNamespaces)

	// If the cookie namespace no longer exists, set currentNamespace to the first existing one.
	if !namespaceExists {
		setCurrentNamespaceInCookie(allNamespaces[0].Name, "currentNamespace", c)
	}
	rest := otherNamespaces(cookie.Value, allNamespaces)

	return cookie.Value, rest, nil
}

// Index handles the dashboard's / (root) endpoint. It returns the dashboard itself.
func (hc ApplicationsController) Index(c *gin.Context) {
	currentNamespace, otherNamespaces, err := getNamespaces(c)
	if handleError(c, err) {
		return
	}

	if currentNamespace == "" {
		// TODO: Redirect to create namespace page. No namespace exists.
		panic("no current namespace")
	}

	// TODO: Move namespace specific links to a left navigation bar and keep only
	// namespace specific actions at the top navbar
	data := map[string]interface{}{
		"currentNamespace": currentNamespace,
		"namespaces":       otherNamespaces,
	}
	Render([]string{
		"main_layout",
		"icons",
		"modals",
		"applications_index",
	}, c, data)
}

// Render renders the given templates into HTML using the provided
// data and returns the result via the provided ResponseWriter.
func Render(templates []string, c *gin.Context, data map[string]interface{}) {
	var viewsDir http.FileSystem
	if os.Getenv("LOCAL_FILESYSTEM") == "true" {
		viewsDir = http.Dir(path.Join(".", "assets", "embedded-web-files", "views"))
	} else {
		viewsDir = filesystem.Views()
	}

	var err error
	tmpl := template.New("page_template")
	tmpl = tmpl.Delims("[[", "]]")
	for _, template := range templates {
		tmplFile, err := viewsDir.Open("/" + template + ".html")
		if err != nil {
			if handleError(c, err) {
				return
			}
		}
		tmplContent, err := ioutil.ReadAll(tmplFile)
		if err != nil {
			if handleError(c, err) {
				return
			}
		}

		tmpl, err = tmpl.Parse(string(tmplContent))
		if err != nil {
			if handleError(c, err) {
				return
			}
		}
	}

	if handleError(c, err) {
		return
	}

	// Inject the request into the data for the template. It can
	// be used in the templates to find the current url.
	data["request"] = c.Request

	var result bytes.Buffer

	err = tmpl.ExecuteTemplate(&result, "main_layout", data)
	if handleError(c, err) {
		return
	}

	c.Data(http.StatusOK, "text/html", result.Bytes())
}

// handleError is a helper which writes the error (if any) to the
// response writer and returns true if there was an error
func handleError(c *gin.Context, err error) bool {
	if err != nil {
		// When attempting to set an error into the response
		// caused an error, give up. The recovery middleware
		// will catch our panic and return that error.
		err := c.AbortWithError(http.StatusInternalServerError, err)
		if err != nil {
			panic(err.Error())
		}
		return true
	}
	return false
}
