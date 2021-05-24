package tailer

import (
	"context"
	"fmt"
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

// FetchLogs writes all the logs of the matching containers to the logChan.
// If ctx is Done() the method stops even if not all logs are fetched.
func FetchLogs(ctx context.Context, logChan chan ContainerLogLine, config *Config, cluster *kubernetes.Cluster) error {
	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return errors.New("no namespace set for tailing logs")
	}

	podList, err := cluster.Kubectl.CoreV1().Pods(namespace).List(
		ctx, metav1.ListOptions{LabelSelector: config.LabelSelector.String()})
	if err != nil {
		return err
	}

	tails := []*Tail{}
	newTail := func(pod corev1.Pod, c corev1.Container) *Tail {
		return NewTail(pod.Namespace, pod.Name, c.Name,
			tracelog.NewLogger().WithName("log-tracing"),
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
	for _, pod := range podList.Items {
		for _, c := range pod.Spec.Containers {
			tails = append(tails, newTail(pod, c))
		}
		for _, c := range pod.Spec.InitContainers {
			tails = append(tails, newTail(pod, c))
		}
	}

	var wg sync.WaitGroup
	for _, t := range tails {
		wg.Add(1)
		go func(tail *Tail) {
			err := tail.Start(ctx, logChan, false)
			if err != nil {
				// TODO: just print it? With a logger?
				fmt.Println(err.Error())
			}
			wg.Done()
		}(t)
	}

	wg.Wait()

	return nil
}

func StreamLogs(ctx context.Context, logChan chan ContainerLogLine, config *Config, cluster *kubernetes.Cluster) error {
	var namespace string
	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return errors.New("no namespace set for tailing logs")
	}

	added, removed, err := Watch(ctx, cluster.Kubectl.CoreV1().Pods(namespace),
		config.PodQuery, config.ContainerQuery, config.ExcludeContainerQuery, config.ContainerState, config.LabelSelector)
	if err != nil {
		return errors.Wrap(err, "failed to set up watch")
	}

	tails := make(map[string]*Tail)
	var wg sync.WaitGroup
	for {
		select {
		case p := <-added:
			id := p.GetID()
			if tails[id] != nil {
				break
			}

			tail := NewTail(p.Namespace, p.Pod, p.Container,
				tracelog.NewLogger().WithName("log-tracing"),
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
			go func() {
				err := tail.Start(ctx, logChan, true)
				if err != nil {
					// TODO: just print it? With a logger?
					fmt.Println(err.Error())
				}
				wg.Done()
			}()
		case p := <-removed:
			id := p.GetID()
			if tails[id] == nil {
				break
			}
			delete(tails, id)
		case <-ctx.Done():
			wg.Wait()
			return nil
		}
	}
}
