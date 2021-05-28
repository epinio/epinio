package tailer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Tail struct {
	Namespace     string
	PodName       string
	ContainerName string
	Options       *TailOptions
	logger        logr.Logger
	clientSet     *kubernetes.Clientset
}

type TailOptions struct {
	Timestamps   bool
	Follow       bool
	SinceSeconds int64
	Exclude      []*regexp.Regexp
	Include      []*regexp.Regexp
	Namespace    bool
	TailLines    *int64
	Logger       logr.Logger
}

// NewTail returns a new tail for a Kubernetes container inside a pod
func NewTail(namespace, podName, containerName string, logger logr.Logger, clientSet *kubernetes.Clientset, options *TailOptions) *Tail {
	return &Tail{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Options:       options,
		logger:        logger,
		clientSet:     clientSet,
	}
}

// Start writes log lines to the logChan. It can be stopped using the ctx.
// It's the caller's responsibility to close the logChan (because there may be more
// instances of this method (go routines) writing to the same channel.
func (t *Tail) Start(ctx context.Context, logChan chan ContainerLogLine, follow bool) error {
	t.logger.Info("starting the tail for pod " + t.PodName)
	var m string
	if t.Options.Namespace {
		m = fmt.Sprintf("Now tracking %s %s › %s ", t.Namespace, t.PodName, t.ContainerName)
	} else {
		m = fmt.Sprintf("Now tracking %s › %s ", t.PodName, t.ContainerName)
	}
	t.logger.Info(m)

	req := t.clientSet.CoreV1().Pods(t.Namespace).GetLogs(t.PodName, &corev1.PodLogOptions{
		Follow:       follow,
		Timestamps:   t.Options.Timestamps,
		Container:    t.ContainerName,
		SinceSeconds: &t.Options.SinceSeconds,
		TailLines:    t.Options.TailLines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	reader := bufio.NewReader(stream)

OUTER:
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				t.logger.Info("reached end of file while tailing container logs")
			} else {
				t.logger.Error(err, "reading failed")
			}
			return nil
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

		logChan <- ContainerLogLine{
			Message:       str,
			ContainerName: t.ContainerName,
			PodName:       t.PodName,
			Namespace:     t.Namespace,
		}
	}
}
