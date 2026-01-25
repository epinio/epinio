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

// tailer manages objects which tail the logs of a collection of pods specified by a label selector.
// This is similar to what the cli tool `stern` does.
package tailer

import (
	"context"
	"regexp"
	"sync"
	"text/template"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type Config struct {
	Namespace             string           // Name of the namespace to monitor
	PodQuery              *regexp.Regexp   // Limit monitoring to pods matching the RE
	Timestamps            bool             // Print timestamps before each entry.
	ContainerQuery        *regexp.Regexp   // Limit monitoring to containers matching the RE
	ExcludeContainerQuery *regexp.Regexp   // Exclusion list if the above alone is not enough.
	ContainerState        ContainerState   // Limit monitoring to containers in this state.
	Exclude               []*regexp.Regexp // If specified suppress all log entries matching the RE
	Include               []*regexp.Regexp // If specified show only log entries matching this RE
	Since                 time.Duration    // Show only log entries younger than the duration.
	SinceTime             *time.Time       // Show only log entries newer than the time.
	AllNamespaces         bool
	LabelSelector         labels.Selector
	TailLines             *int64
	Template              *template.Template // Template to apply to log entries for formatting
	Ordered               bool               // Featch/stream logs in container order, synchronously
}

// ContainerLogLine is an object that represents a line from the logs of a container.
// It is what the Run() method returns through a channel
type ContainerLogLine struct {
	Message       string
	ContainerName string
	PodName       string
	Namespace     string
	Timestamp     string
}

// FetchLogs writes all the logs of the matching containers to the logChan.
// If ctx is Done() the method stops even if not all logs are fetched.
func FetchLogs(
	ctx context.Context,
	logChan chan ContainerLogLine,
	wg *sync.WaitGroup,
	config *Config,
	cluster *kubernetes.Cluster,
) error {
	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return errors.New("no namespace set for tailing logs")
	}

	helpers.Logger.Infow("list pods")
	podList, err := cluster.Kubectl.CoreV1().Pods(namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: config.LabelSelector.String()},
	)
	if err != nil {
		return err
	}

	tailOptions := &TailOptions{
		Timestamps: config.Timestamps,
		SinceTime:  config.SinceTime,
		Exclude:    config.Exclude,
		Include:    config.Include,
		Namespace:  config.AllNamespaces,
		TailLines:  config.TailLines,
	}

	// If no TailLines is set, or it is set to 0, override it to the max value
	if config.TailLines == nil || (config.TailLines != nil && *config.TailLines == 0) {
		tailOverride := int64(100000)
		tailOptions.TailLines = &tailOverride
	}

	// Set SinceSeconds to 2 days if no other value is set
	if config.Since != 0 {
		tailOptions.SinceSeconds = int64(config.Since.Seconds())
	} else {
		tailOptions.SinceSeconds = int64(172800)
	}

	// SinceTime overrides SinceSeconds
	if config.SinceTime != nil {
		tailOptions.SinceSeconds = 0
		tailOptions.SinceTime = config.SinceTime
	}

	tails := []*Tail{}
	newTail := func(pod corev1.Pod, c corev1.Container) *Tail {
		// Convert zap logger to logr for tailer functions (compatibility bridge for logr.Logger interface)
		return NewTail(pod.Namespace, pod.Name, c.Name,
			helpers.SugaredLoggerToLogr(helpers.Logger.With("component", "log-tracing")).V(4),
			cluster.Kubectl,
			tailOptions,
		)
	}

	acceptable := func(c corev1.Container) bool {
		if !config.ContainerQuery.MatchString(c.Name) {
			return false
		}
		if config.ExcludeContainerQuery != nil &&
			config.ExcludeContainerQuery.MatchString(c.Name) {
			return false
		}
		return true
	}

	helpers.Logger.Infow("filter pods, containers")

	for _, pod := range podList.Items {
		for _, c := range pod.Spec.InitContainers {
			if !acceptable(c) {
				continue
			}
			tails = append(tails, newTail(pod, c))

			helpers.Logger.Debugw("have init container",
				"namespace", pod.Namespace,
				"pod", pod.Name,
				"container", c.Name,
			)
		}
		for _, c := range pod.Spec.Containers {
			if !acceptable(c) {
				continue
			}
			tails = append(tails, newTail(pod, c))

			helpers.Logger.Debugw("have container",
				"namespace", pod.Namespace,
				"pod", pod.Name,
				"container", c.Name,
			)
		}
	}

	if config.Ordered {
		helpers.Logger.Debugw("fetch in order")

		for _, t := range tails {
			helpers.Logger.Debugw("tail container",
				"namespace", t.Namespace,
				"pod", t.PodName,
				"container", t.ContainerName,
			)

			err := t.Start(ctx, logChan, false)
			if err != nil {
				helpers.Logger.Errorw("failed to start a Tail", "error", err)
			}
		}

		return nil
	}

	for _, t := range tails {
		helpers.Logger.Debugw("tail container",
			"namespace", t.Namespace,
			"pod", t.PodName,
			"container", t.ContainerName,
		)

		wg.Add(1)
		go func(tail *Tail) {
			err := tail.Start(ctx, logChan, false)
			if err != nil {
				helpers.Logger.Errorw("failed to start a Tail", "error", err)
			}
			wg.Done()
		}(t)
	}

	return nil
}

// StreamLogs writes the logs of all matching containers to the
// logChan.  The containers are determined by an internal watcher
// polling the cluster for pod __changes__.
func StreamLogs(ctx context.Context, logChan chan ContainerLogLine, wg *sync.WaitGroup, config *Config, cluster *kubernetes.Cluster) error {
	// Convert zap logger to logr for tailer functions (compatibility bridge for logr.Logger interface)
	logger := helpers.SugaredLoggerToLogr(helpers.Logger.With("component", "tail-handling")).V(3)

	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return errors.New("no namespace set for tailing logs")
	}

	excludedStr := "<none>"
	if config.ExcludeContainerQuery != nil {
		excludedStr = config.ExcludeContainerQuery.String()
	}
	logger.Info("start watcher",
		"pods", config.PodQuery.String(),
		"containers", config.ContainerQuery.String(),
		"excluded", excludedStr,
		"selector", config.LabelSelector.String())

	added, removed, err := Watch(
		ctx,
		cluster.Kubectl.CoreV1().Pods(namespace),
		config.PodQuery,
		config.ContainerQuery,
		config.ExcludeContainerQuery,
		config.ContainerState,
		config.LabelSelector,
	)
	if err != nil {
		return errors.Wrap(err, "failed to set up watch")
	}

	// Process watch reports (added/removed tailer targets)

	tails := make(map[string]*Tail)

	logger.Info("await reports")
	defer func() {
		logger.Info("report processing ends")
	}()
	for {
		select {
		case p := <-added:
			id := p.GetID()
			if tails[id] != nil {
				continue
			}

			logger.Info("tailer add", "id", id)

			// Convert zap logger to logr for tailer functions (compatibility bridge for logr.Logger interface)
			tail := NewTail(p.Namespace, p.Pod, p.Container,
				helpers.SugaredLoggerToLogr(helpers.Logger.With("component", "log-tracing")),
				cluster.Kubectl,
				&TailOptions{
					Timestamps:   config.Timestamps,
					SinceTime:    config.SinceTime,
					SinceSeconds: int64(config.Since.Seconds()),
					Exclude:      config.Exclude,
					Include:      config.Include,
					Namespace:    config.AllNamespaces,
					TailLines:    config.TailLines,
				})
			tails[id] = tail

			wg.Add(1)
			go func(id string) {
				logger.Info("tailer start", "id", id)
				err := tail.Start(ctx, logChan, true)
				if err != nil {
					logger.Error(err, "failed to start a Tail")
				}
				logger.Info("tailer done", "id", id)
				wg.Done()
			}(id)

			logger.Info("tailer added", "id", id)

		case p := <-removed:
			id := p.GetID()
			if tails[id] == nil {
				continue
			}

			logger.Info("tailer remove", "id", id)

			delete(tails, id)

			logger.Info("tailer removed", "id", id)
		case <-ctx.Done():
			logger.Info("received stop request")
			return nil
		}
	}
}
