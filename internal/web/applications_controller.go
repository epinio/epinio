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
	"github.com/epinio/epinio/internal/organizations"
	"github.com/gin-gonic/gin"
)

// ApplicationsController represents all general functionality of the dashboard
type ApplicationsController struct {
}

// setCurrentOrgInCookie is a helper for creating cookies to persist system state in the browser
func setCurrentOrgInCookie(org, cookieName string, c *gin.Context) {
	c.SetCookie(cookieName,
		org,
		365*24*60*60, // 1 year
		"/",
		"",
		false,
		false,
	)
}

// getOrgs tries to decide what the current organization is.
// First looks in the cookie named "currentOrg". If it is exists and the org
// set there still exists that's our current organization. If the cookie exists
// but the org does not (e.g. because it was deleted) or if the cookie does not
// exist, then the first existing org becomes the current org and the cookie is
// updated. If no orgs exist, then an empty string is returned as the org name.
// The function also returns the rest of the available orgs.
func getOrgs(c *gin.Context) (string, []string, error) {

	ctx := c.Request.Context()
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return "", []string{}, err
	}

	orgs, err := organizations.List(ctx, cluster)
	if err != nil {
		return "", []string{}, err
	}
	if len(orgs) == 0 {
		return "", []string{}, nil
	}

	otherOrgs := func(current string, orgs []organizations.Organization) []string {
		otherOrgs := []string{}
		for _, org := range orgs {
			if org.Name != current {
				otherOrgs = append(otherOrgs, org.Name)
			}
		}
		return otherOrgs
	}

	cookie, err := c.Request.Cookie("currentOrg")
	if err != nil {
		// There was no cookie, let's create one
		if err == http.ErrNoCookie {
			currentOrg := orgs[0].Name
			restOrgs := otherOrgs(currentOrg, orgs)
			setCurrentOrgInCookie(currentOrg, "currentOrg", c)
			return currentOrg, restOrgs, nil
		}
		return "", []string{}, err
	}
	orgExists := func(cookieOrg string, orgs []organizations.Organization) bool {
		for _, org := range orgs {
			if org.Name == cookieOrg {
				return true
			}
		}
		return false
	}(cookie.Value, orgs)

	// If the cookie org no longer exists, set currentOrg to the first existing one.
	if !orgExists {
		setCurrentOrgInCookie(orgs[0].Name, "currentOrg", c)
	}
	restOrgs := otherOrgs(cookie.Value, orgs)

	return cookie.Value, restOrgs, nil
}

// Index handles the dashboard's / (root) endpoint. It returns the dashboard itself.
func (hc ApplicationsController) Index(c *gin.Context) {
	currentOrg, otherOrgs, err := getOrgs(c)
	if handleError(c, err) {
		return
	}

	if currentOrg == "" {
		// TODO: Redirect to create org page. No orgs exist.
		panic("no current namespace")
	}

	// TODO: Move org specific links to a left navigation bar and keep only
	// org specific actions at the top navbar
	data := map[string]interface{}{
		"currentOrg": currentOrg,
		"orgs":       otherOrgs,
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
