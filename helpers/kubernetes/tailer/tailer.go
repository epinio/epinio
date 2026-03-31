// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tailer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	SinceTime    *time.Time
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
	var ident string
	var mode string
	if t.Options.Namespace {
		ident = fmt.Sprintf("%s %s › %s", t.Namespace, t.PodName, t.ContainerName)
		mode = "global"
	} else {
		ident = fmt.Sprintf("%s › %s", t.PodName, t.ContainerName)
		mode = "local"
	}

	helpers.Logger.Infow("starting the tail for pod", "pod", t.PodName)

	podLogOptions := &corev1.PodLogOptions{
		Follow:     follow,
		Timestamps: t.Options.Timestamps,
		Container:  t.ContainerName,
		TailLines:  t.Options.TailLines,
	}

	if t.Options.SinceTime != nil {
		podLogOptions.SinceTime = &metav1.Time{Time: *t.Options.SinceTime}
	} else if t.Options.SinceSeconds > 0 {
		podLogOptions.SinceSeconds = &t.Options.SinceSeconds
	}

	// Log the options being passed to Kubernetes API
	helpers.Logger.Infow("calling Kubernetes API with options",
		"tail_lines", t.Options.TailLines,
		"since_seconds", t.Options.SinceSeconds,
		"follow", follow,
		"container", t.ContainerName,
	)

	helpers.Logger.Infow("podLogOptions", "options", podLogOptions)
	req := t.clientSet.CoreV1().Pods(t.Namespace).GetLogs(t.PodName, podLogOptions)

	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err := stream.Close(); err != nil {
			t.logger.Error(err, "failed to close stream: ")
		}
	}()

	reader := bufio.NewReader(stream)

	helpers.Logger.Infow("now tracking", "mode", mode, "ident", ident)
OUTER:
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				helpers.Logger.Infow("tailer reached end of logs", "container", ident)
			} else {
				helpers.Logger.Errorw("reading failed", "error", err)
			}
			return nil
		}

		str := strings.TrimRight(string(line), "\r\n\t ")

		// Parse timestamp if present (RFC3339 format from Kubernetes)
		// Format: 2023-04-15T10:30:45.123456789Z message
		var timestamp string
		var message string
		if t.Options.Timestamps {
			// Timestamps from Kubernetes are in RFC3339Nano format
			// Look for the first space after the timestamp
			spaceIdx := strings.Index(str, " ")
			if spaceIdx > 0 {
				timestamp = str[:spaceIdx]
				message = str[spaceIdx+1:]
			} else {
				// No space found, use the whole line as message
				message = str
			}
		} else {
			message = str
		}

		for _, rex := range t.Options.Exclude {
			if rex.MatchString(message) {
				continue OUTER
			}
		}

		if len(t.Options.Include) != 0 {
			matches := false
			for _, rin := range t.Options.Include {
				if rin.MatchString(message) {
					matches = true
					break
				}
			}
			if !matches {
				continue OUTER
			}
		}

		helpers.Logger.Debugw("passing", "container", ident, "message", message)
		logChan <- ContainerLogLine{
			Message:       message,
			ContainerName: t.ContainerName,
			PodName:       t.PodName,
			Namespace:     t.Namespace,
			Timestamp:     timestamp,
		}
	}
}
