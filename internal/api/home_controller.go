package api

import (
	"html/template"
	"io/ioutil"
	"net/http"

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
	filesystem := filesystem.NewFilesystem(localFilesystem)
	var err error
	tmpl := template.New("page_template")
	tmpl = tmpl.Delims("[[", "]]")
	for _, template := range templates {
		tmplFile, err := filesystem.Open("views/" + template + ".html")
		if err != nil {
			break
		}
		tmplContent, err := ioutil.ReadAll(tmplFile)
		if err != nil {
			break
		}

		tmpl, err = tmpl.Parse(string(tmplContent))
		if err != nil {
			break
		}
	}

	if handleError(w, err, 500) {
		return
	}
	w.WriteHeader(http.StatusOK)
	tmpl.ExecuteTemplate(w, "main_layout", data)
}

// Write the error to the response writer and return  true if there was an error
func handleError(w http.ResponseWriter, err error, code int) bool {
	if err != nil {
		http.Error(w, err.Error(), 500)
		return true
	}
	return false
}
