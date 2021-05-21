package tailer

import (
	"context"
	"regexp"
	"sync"
	"text/template"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Config contains the config for stern
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
}

// ContainerLogLine is an object that represents a line from the logs of a container.
// It is what the Run() method returns through a channel
type ContainerLogLine struct {
	Message       string
	ContainerName string
	PodName       string
	Namespace     string
}

// Notes on the above:
//   - For containers `ContainerQuery` is applied before
//     `ExcludeContainerQuery`. IOW use `CQ` to get an initial list of
//     containers and then use `ECQ` to pare that down further.
//
//   - For log entries `Exclude` is applied before `Include`.

// Run returns a channel of ContainerLogLine.
// Depending on the value of the "follow" argument, the channel may stay open until the consumer
// closes the context or until there are no containers left to stream logs.
func Run(ctx context.Context, follow bool, config *Config, cluster *kubernetes.Cluster) (chan ContainerLogLine, error) {
	var result chan ContainerLogLine
	var err error
	if !follow {
		result, err = RunUntilStopped(ctx, config, cluster)
	} else {
		result, err = RunUntilNoMoreLogs(ctx, config, cluster)
	}

	return result, err
}

// RunUntilStopped will keep watching for matching containers and stream their
// logs to the ContainerLogLine channel until ctx is Done(). The channel will
// be closed then that happens.
func RunUntilStopped(ctx context.Context, config *Config, cluster *kubernetes.Cluster) (chan ContainerLogLine, error) {
	var wg sync.WaitGroup

	containerLogsChan := make(chan ContainerLogLine, 10)

	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return nil, errors.New("no namespace set for tailing logs")
	}

	podList, err := cluster.Kubectl.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: config.LabelSelector.String()})
	if err != nil {
		return nil, err
	}
	addTail := func(pod corev1.Pod, c corev1.Container) {
		tail := NewTail(containerLogsChan, pod.Namespace, pod.Name, c.Name, config.Template,
			tracelog.NewLogger().WithName("log-tracing"),
			&TailOptions{
				Timestamps:   config.Timestamps,
				SinceSeconds: int64(config.Since.Seconds()),
				Exclude:      config.Exclude,
				Include:      config.Include,
				Namespace:    config.AllNamespaces,
				TailLines:    config.TailLines,
			})

		wg.Add(1)
		go func() {
			tail.Start(ctx, cluster.Kubectl.CoreV1().Pods(pod.Namespace))
			wg.Done()
		}()
	}
	for _, pod := range podList.Items {
		for _, c := range pod.Spec.Containers {
			addTail(pod, c)
		}
		for _, c := range pod.Spec.InitContainers {
			addTail(pod, c)
		}
	}

	go func() {
		// If ctx is Done() then the go routines will stop themselves and this will
		// close the channel too. The channel is also closed when all go routines
		// return.
		wg.Wait()
		close(containerLogsChan)
	}()

	return containerLogsChan, nil
}

// RunUntilNoMoreLogs stream the logs of all the matching containers (without
// follow) and close the channel when it's done.
func RunUntilNoMoreLogs(ctx context.Context, config *Config, cluster *kubernetes.Cluster) (chan ContainerLogLine, error) {
	containerLogsChan := make(chan ContainerLogLine, 10)
	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return nil, errors.New("no namespace set for tailing logs")
	}

	added, removed, err := Watch(ctx, cluster.Kubectl.CoreV1().Pods(namespace), config.PodQuery, config.ContainerQuery, config.ExcludeContainerQuery, config.ContainerState, config.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up watch")
	}

	tails := make(map[string]*Tail)

	go func() {
		for p := range added {
			id := p.GetID()
			if tails[id] != nil {
				continue
			}

			tail := NewTail(containerLogsChan, p.Namespace, p.Pod, p.Container, config.Template,
				tracelog.NewLogger().WithName("log-tracing"),
				&TailOptions{
					Timestamps:   config.Timestamps,
					SinceSeconds: int64(config.Since.Seconds()),
					Exclude:      config.Exclude,
					Include:      config.Include,
					Namespace:    config.AllNamespaces,
					TailLines:    config.TailLines,
				})
			tails[id] = tail

			tail.Start(ctx, cluster.Kubectl.CoreV1().Pods(p.Namespace))
		}
	}()

	go func() {
		for p := range removed {
			id := p.GetID()
			if tails[id] == nil {
				continue
			}
			tails[id].Close()
			delete(tails, id)
		}
	}()

	return containerLogsChan, nil
}
