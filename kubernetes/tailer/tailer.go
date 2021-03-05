package tailer

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/suse/carrier/paas/ui"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type Tail struct {
	Namespace      string
	PodName        string
	Origin         string
	ContainerName  string
	Options        *TailOptions
	req            *rest.Request
	closed         chan struct{}
	podColor       *color.Color
	containerColor *color.Color
	tmpl           *template.Template
	ui             *ui.UI
}

type TailOptions struct {
	Timestamps   bool
	SinceSeconds int64
	Exclude      []*regexp.Regexp
	Include      []*regexp.Regexp
	Namespace    bool
	TailLines    *int64
}

// NewTail returns a new tail for a Kubernetes container inside a pod
func NewTail(ui *ui.UI, namespace, podName, containerName string, tmpl *template.Template, options *TailOptions) *Tail {
	return &Tail{
		Namespace:     namespace,
		PodName:       podName,
		Origin:        originOf(podName),
		ContainerName: containerName,
		Options:       options,
		closed:        make(chan struct{}),
		tmpl:          tmpl,
		ui:            ui,
	}
}

var colorList = [][2]*color.Color{
	{color.New(color.FgHiCyan), color.New(color.FgCyan)},
	{color.New(color.FgHiGreen), color.New(color.FgGreen)},
	{color.New(color.FgHiMagenta), color.New(color.FgMagenta)},
	{color.New(color.FgHiYellow), color.New(color.FgYellow)},
	{color.New(color.FgHiBlue), color.New(color.FgBlue)},
	{color.New(color.FgHiRed), color.New(color.FgRed)},
}

func determineColor(podName string) (podColor, containerColor *color.Color) {
	hash := fnv.New32()
	hash.Write([]byte(podName))
	idx := hash.Sum32() % uint32(len(colorList))

	colors := colorList[idx]
	return colors[0], colors[1]
}

func originOf(podName string) string {
	if strings.HasPrefix(podName, "staging-pipeline-run-") {
		return "[STAGE]"
	}
	return "[APP]"
}

// Start starts tailing
func (t *Tail) Start(ctx context.Context, i v1.PodInterface) {
	t.podColor, t.containerColor = determineColor(t.Origin)

	go func() {
		g := color.New(color.FgHiGreen, color.Bold).SprintFunc()
		p := t.podColor.SprintFunc()
		c := t.containerColor.SprintFunc()
		var m string
		if t.Options.Namespace {
			m = fmt.Sprintf("%s %s %s › %s ", g("Now tracking"), p(t.Namespace), p(t.PodName), c(t.ContainerName))
		} else {
			m = fmt.Sprintf("%s %s › %s ", g("Now tracking"), p(t.PodName), c(t.ContainerName))
		}
		t.ui.ProgressNote().V(1).KeepLine().Msg(m)

		req := i.GetLogs(t.PodName, &corev1.PodLogOptions{
			Follow:       true,
			Timestamps:   t.Options.Timestamps,
			Container:    t.ContainerName,
			SinceSeconds: &t.Options.SinceSeconds,
			TailLines:    t.Options.TailLines,
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			if context.Canceled == nil {
				fmt.Println(errors.Wrapf(err, "Error opening stream to %s/%s: %s\n", t.Namespace, t.PodName, t.ContainerName))
			}
			return
		}
		defer stream.Close()

		go func() {
			<-t.closed
			stream.Close()
		}()

		reader := bufio.NewReader(stream)

	OUTER:
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}

			str := strings.TrimRight(string(line), "\r\n\t ")

			for _, rex := range t.Options.Exclude {
				if rex.MatchString(str) {
					continue OUTER
				}
			}

			if len(t.Options.Include) != 0 {
				matches := false
				for _, rin := range t.Options.Include {
					if rin.MatchString(str) {
						matches = true
						break
					}
				}
				if !matches {
					continue OUTER
				}
			}

			t.Print(str)
		}
	}()

	go func() {
		<-ctx.Done()
		close(t.closed)
	}()
}

// Close stops tailing
func (t *Tail) Close() {
	r := color.New(color.FgHiRed, color.Bold).SprintFunc()
	p := t.podColor.SprintFunc()
	if t.Options.Namespace {
		fmt.Fprintf(os.Stderr, "%s %s %s\n", r("-"), p(t.Namespace), p(t.PodName))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s\n", r("-"), p(t.PodName))
	}
	close(t.closed)
}

// Print prints a color coded log message with the pod and container names
func (t *Tail) Print(msg string) {
	vm := Log{
		Message:        msg,
		Namespace:      t.Namespace,
		PodName:        t.PodName,
		Origin:         t.Origin,
		ContainerName:  t.ContainerName,
		PodColor:       t.podColor,
		ContainerColor: t.containerColor,
	}

	var result bytes.Buffer
	err := t.tmpl.Execute(&result, vm)
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("expanding template failed: %s", err))
		return
	}
	t.ui.ProgressNote().V(1).KeepLine().Msg(result.String() + " ")
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

	// Origin
	Origin string `json:"origin"`

	// ContainerName of the container
	ContainerName string `json:"containerName"`

	PodColor       *color.Color `json:"-"`
	ContainerColor *color.Color `json:"-"`
}
