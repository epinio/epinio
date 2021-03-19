package api

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/suse/carrier/internal/filesystem"
)

type HomeController struct {
}

func (hc HomeController) Index(w http.ResponseWriter, r *http.Request) {
	Render([]string{"main_layout", "icons", "modals", "home"}, w, nil)
}

// Render renders the given templates using the provided data and writes the result
// to the provided ResponseWriter.
func Render(templates []string, w http.ResponseWriter, data interface{}) {
	var viewsDir http.FileSystem
	if localFilesystem {
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
