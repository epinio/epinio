package web

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	giteaSDK "code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/filesystem"
)

type ApplicationsController struct {
}

// getOrgs tries to decide what the current organization is.
// First looks in the cookie named "currentOrg". If it is exists and the org
// set there still exists that's our current organization. If the cookie exists
// but the org does not (e.g. because it was deleted) or if the cookie does not
// exist, then the first existing org becomes the current org and the cookie is
// updated. If no orgs exist, then an empty string is returned as the org name.
// The function also returns the rest of the available orgs.
func getOrgs(w http.ResponseWriter, r *http.Request) (string, []string, error) {
	gitea, err := clients.GetGiteaClient()
	if err != nil {
		return "", []string{}, err
	}

	orgs, _, err := gitea.Client.AdminListOrgs(giteaSDK.AdminListOrgsOptions{})
	if err != nil {
		return "", []string{}, err
	}
	if len(orgs) == 0 {
		return "", []string{}, nil
	}

	otherOrgs := func(current string, orgs []*giteaSDK.Organization) []string {
		orgNames := []string{}
		for _, org := range orgs {
			if org.UserName != current {
				orgNames = append(orgNames, org.UserName)
			}
		}
		return orgNames
	}

	cookie, err := r.Cookie("currentOrg")
	if err != nil {
		// There was no cookie, let's create one
		if err == http.ErrNoCookie {
			currentOrg := orgs[0].UserName
			restOrgs := otherOrgs(currentOrg, orgs)
			expiration := time.Now().Add(365 * 24 * time.Hour)
			cookie := http.Cookie{Name: "currentOrg", Value: currentOrg, Expires: expiration}
			http.SetCookie(w, &cookie)
			return currentOrg, restOrgs, nil
		} else {
			return "", []string{}, err
		}
	}
	orgExists := func(cookieOrg string, orgs []*giteaSDK.Organization) bool {
		for _, org := range orgs {
			if org.UserName == cookieOrg {
				return true
			}
		}
		return false
	}(cookie.Value, orgs)

	// If the cookie org no longer exists, set currentOrg to the first existing one.
	if !orgExists {
		cookie.Value = orgs[0].UserName
		http.SetCookie(w, cookie)
	}
	restOrgs := otherOrgs(cookie.Value, orgs)

	return cookie.Value, restOrgs, nil
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) {
	currentOrg, otherOrgs, err := getOrgs(w, r)
	if handleError(w, err, http.StatusInternalServerError) {
		return
	}

	if currentOrg == "" {
		// TODO: Redirect to create org page. No orgs exist.
		panic("no current org")
	}

	// TODO: Move org specific links to a left navigation bar and keep only
	// org specific actions at the top navbar
	data := map[string]interface{}{
		"currentOrg": currentOrg,
		"orgs":       otherOrgs,
	}
	Render([]string{"main_layout", "icons", "applications_index"}, w, r, data)
}

// Render renders the given templates using the provided data and writes the result
// to the provided ResponseWriter.
func Render(templates []string, w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
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
			if handleError(w, err, 500) {
				return
			}
		}
		tmplContent, err := ioutil.ReadAll(tmplFile)
		if err != nil {
			if handleError(w, err, 500) {
				return
			}
		}

		tmpl, err = tmpl.Parse(string(tmplContent))
		if err != nil {
			if handleError(w, err, 500) {
				return
			}
		}
	}

	if handleError(w, err, 500) {
		return
	}
	w.WriteHeader(http.StatusOK)

	// Inject the request in the data. It can be used in the templates to find the
	// current url
	data["request"] = r
	err = tmpl.ExecuteTemplate(w, "main_layout", data)
	if handleError(w, err, 500) {
		return
	}
}

// Write the error to the response writer and return  true if there was an error
func handleError(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		http.Error(w, err.Error(), 500)
		return true
	}
	return false
}
