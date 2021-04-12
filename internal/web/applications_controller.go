package web

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/epinio/epinio/internal/filesystem"
)

type ApplicationsController struct {
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) {
	Render([]string{"main_layout", "icons", "applications_index"}, w, r, map[string]interface{}{})
}

// Render renders the given templates using the provided data and writes the result
// to the provided ResponseWriter.
func Render(templates []string, w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	var viewsDir http.FileSystem
	if os.Getenv("LOCAL_FILESYSTEM") == "true" {
		viewsDir = http.Dir(path.Join(".", "embedded-web-files", "views"))
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
