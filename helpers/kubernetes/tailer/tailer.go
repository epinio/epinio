// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
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
	var ident string
	var mode string
	if t.Options.Namespace {
		ident = fmt.Sprintf("%s %s › %s", t.Namespace, t.PodName, t.ContainerName)
		mode = "global"
	} else {
		ident = fmt.Sprintf("%s › %s", t.PodName, t.ContainerName)
		mode = "local"
	}

	t.logger.Info("starting the tail for pod " + t.PodName)

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

	t.logger.Info(fmt.Sprintf("now tracking %s %s", mode, ident))
OUTER:
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				t.logger.Info("tailer reached end of logs", "container", ident)
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

		t.logger.Info("passing", "container", ident, "", str)
		logChan <- ContainerLogLine{
			Message:       str,
			ContainerName: t.ContainerName,
			PodName:       t.PodName,
			Namespace:     t.Namespace,
		}
	}
}
