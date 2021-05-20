package tailer

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type Tail struct {
	Namespace     string
	PodName       string
	ContainerName string
	Options       *TailOptions
	closed        chan struct{}
	logger        logr.Logger
	logChan       chan ContainerLogLine
}

type TailOptions struct {
	Timestamps   bool
	SinceSeconds int64
	Exclude      []*regexp.Regexp
	Include      []*regexp.Regexp
	Namespace    bool
	TailLines    *int64
	Logger       logr.Logger
}

// NewTail returns a new tail for a Kubernetes container inside a pod
func NewTail(logChan chan ContainerLogLine, namespace, podName, containerName string, tmpl *template.Template, logger logr.Logger, options *TailOptions) *Tail {
	return &Tail{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Options:       options,
		closed:        make(chan struct{}),
		logChan:       logChan,
		logger:        logger,
	}
}

// Start starts tailing
func (t *Tail) Start(ctx context.Context, i v1.PodInterface) {
	go func() {
		var m string
		if t.Options.Namespace {
			m = fmt.Sprintf("Now tracking %s %s › %s ", t.Namespace, t.PodName, t.ContainerName)
		} else {
			m = fmt.Sprintf("Now tracking %s › %s ", t.PodName, t.ContainerName)
		}
		t.logger.Info(m)

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

			t.logChan <- ContainerLogLine{
				Message:       str,
				ContainerName: t.ContainerName,
				PodName:       t.PodName,
				Namespace:     t.Namespace,
			}
		}
	}()

	go func() {
		<-ctx.Done()
		defer func() {
			_ = recover() // Ignore the case when t.closed is already closed (race conditions)
		}()

		t.Close()
	}()
}

// Close stops tailing
func (t *Tail) Close() {
	close(t.closed)
}
