package tailer

import (
	"encoding/json"
	"text/template"

	"github.com/fatih/color"
	"github.com/pkg/errors"
)

// DefaultSingleNamespaceTemplate returns a printing template used when
// printing with colors and watching resources in a single namespace
func DefaultSingleNamespaceTemplate() *template.Template {
	t := "{{color .PodColor .PodName}} {{color .ContainerColor .ContainerName}} {{.Message}}"

	funs := map[string]interface{}{
		"json": func(in interface{}) (string, error) {
			b, err := json.Marshal(in)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},
	}
	template, err := template.New("log").Funcs(funs).Parse(t)
	if err != nil {
		panic(errors.Wrap(err, "unable to parse template"))
	}

	return template
}
