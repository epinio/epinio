package web

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/filesystem"
)

type ApplicationsController struct {
}

func setCurrentOrgInCookie(org, cookieName string, w http.ResponseWriter) error {
	cookie := http.Cookie{
		Name:    cookieName,
		Value:   org,
		Path:    "/",
		Expires: time.Now().Add(365 * 24 * time.Hour),
	}
	http.SetCookie(w, &cookie)

	return nil
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

	orgNames, err := gitea.OrgNames()
	if err != nil {
		return "", []string{}, err
	}
	if len(orgNames) == 0 {
		return "", []string{}, nil
	}

	otherOrgs := func(current string, orgNames []string) []string {
		otherOrgs := []string{}
		for _, org := range orgNames {
			if org != current {
				otherOrgs = append(otherOrgs, org)
			}
		}
		return otherOrgs
	}

	cookie, err := r.Cookie("currentOrg")
	if err != nil {
		// There was no cookie, let's create one
		if err == http.ErrNoCookie {
			currentOrg := orgNames[0]
			restOrgs := otherOrgs(currentOrg, orgNames)
			setCurrentOrgInCookie(currentOrg, "currentOrg", w)
			return currentOrg, restOrgs, nil
		} else {
			return "", []string{}, err
		}
	}
	orgExists := func(cookieOrg string, orgNames []string) bool {
		for _, org := range orgNames {
			if org == cookieOrg {
				return true
			}
		}
		return false
	}(cookie.Value, orgNames)

	// If the cookie org no longer exists, set currentOrg to the first existing one.
	if !orgExists {
		setCurrentOrgInCookie(orgNames[0], "currentOrg", w)
	}
	restOrgs := otherOrgs(cookie.Value, orgNames)

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
