// Package logprinter is used to print container log lines in color
package logprinter

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"os"
	"text/template"

	"github.com/epinio/epinio/helpers/termui"
	"github.com/fatih/color"
)

var colorList = [][2]*color.Color{
	{color.New(color.FgHiCyan), color.New(color.FgCyan)},
	{color.New(color.FgHiGreen), color.New(color.FgGreen)},
	{color.New(color.FgHiMagenta), color.New(color.FgMagenta)},
	{color.New(color.FgHiYellow), color.New(color.FgYellow)},
	{color.New(color.FgHiBlue), color.New(color.FgBlue)},
	{color.New(color.FgHiRed), color.New(color.FgRed)},
}

type LogPrinter struct {
	Tmpl *template.Template
}

// Log is the object which will be used together with the template to generate
// the output.
type Log struct {
	// Message is the log message itself
	Message string `json:"message"`

	// Namespace of the pod
	Namespace string `json:"namespace"`

	// PodName of the pod
	PodName string `json:"podName"`

	// ContainerName of the container
	ContainerName string `json:"containerName"`

	PodColor       *color.Color `json:"-"`
	ContainerColor *color.Color `json:"-"`
}

func (printer LogPrinter) Print(log Log, uiMsg *termui.Message) {
	log.PodColor, log.ContainerColor = determineColor(log.PodName)

	var result bytes.Buffer
	err := printer.Tmpl.Execute(&result, log)
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("expanding template failed: %s", err))
		return
	}

	uiMsg.Msg(result.String() + " ")
}

func determineColor(podName string) (podColor, containerColor *color.Color) {
	hash := fnv.New32()
	hash.Write([]byte(podName))
	idx := hash.Sum32() % uint32(len(colorList))

	colors := colorList[idx]
	return colors[0], colors[1]
}
