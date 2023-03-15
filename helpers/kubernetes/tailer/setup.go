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

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
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
}

// FetchLogs writes all the logs of the matching containers to the logChan.
// If ctx is Done() the method stops even if not all logs are fetched.
func FetchLogs(ctx context.Context, logChan chan ContainerLogLine, wg *sync.WaitGroup, config *Config, cluster *kubernetes.Cluster) error {
	logger := requestctx.Logger(ctx).WithName("fetching-logs").V(3)
	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return errors.New("no namespace set for tailing logs")
	}

	logger.Info("list pods")
	podList, err := cluster.Kubectl.CoreV1().Pods(namespace).List(
		ctx, metav1.ListOptions{LabelSelector: config.LabelSelector.String()})
	if err != nil {
		return err
	}

	tails := []*Tail{}
	newTail := func(pod corev1.Pod, c corev1.Container) *Tail {
		return NewTail(pod.Namespace, pod.Name, c.Name,
			requestctx.Logger(ctx).WithName("log-tracing").V(4),
			cluster.Kubectl,
			&TailOptions{
				Timestamps:   config.Timestamps,
				SinceSeconds: int64(config.Since.Seconds()),
				Exclude:      config.Exclude,
				Include:      config.Include,
				Namespace:    config.AllNamespaces,
				TailLines:    config.TailLines,
			})
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

	logger.Info("filter pods, containers")

	for _, pod := range podList.Items {
		for _, c := range pod.Spec.InitContainers {
			if !acceptable(c) {
				continue
			}
			tails = append(tails, newTail(pod, c))

			logger.Info("have", "namespace", pod.Namespace, "pod", pod.Name, "container", c.Name)
		}
		for _, c := range pod.Spec.Containers {
			if !acceptable(c) {
				continue
			}
			tails = append(tails, newTail(pod, c))

			logger.Info("have", "namespace", pod.Namespace, "pod", pod.Name, "container", c.Name)
		}
	}

	if config.Ordered {
		logger.Info("fetch in order")

		for _, t := range tails {
			logger.Info("tail", "namespace", t.Namespace, "pod", t.PodName, "container", t.ContainerName)

			err := t.Start(ctx, logChan, false)
			if err != nil {
				logger.Error(err, "failed to start a Tail")
			}
		}

		return nil
	}

	for _, t := range tails {
		logger.Info("tail", "namespace", t.Namespace, "pod", t.PodName, "container", t.ContainerName)

		wg.Add(1)
		go func(tail *Tail) {
			err := tail.Start(ctx, logChan, false)
			if err != nil {
				logger.Error(err, "failed to start a Tail")
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
	logger := requestctx.Logger(ctx).WithName("tail-handling").V(3)

	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return errors.New("no namespace set for tailing logs")
	}

	logger.Info("start watcher",
		"pods", config.PodQuery.String(),
		"containers", config.ContainerQuery.String(),
		"excluded", config.ExcludeContainerQuery.String(),
		"selector", config.LabelSelector.String())
	added, removed, err := Watch(ctx, cluster.Kubectl.CoreV1().Pods(namespace),
		config.PodQuery, config.ContainerQuery, config.ExcludeContainerQuery, config.ContainerState, config.LabelSelector)
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

			tail := NewTail(p.Namespace, p.Pod, p.Container,
				requestctx.Logger(ctx).WithName("log-tracing"),
				cluster.Kubectl,
				&TailOptions{
					Timestamps:   config.Timestamps,
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
