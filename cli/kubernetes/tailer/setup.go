package tailer

import (
	"context"
	"regexp"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/kubernetes"
	"k8s.io/apimachinery/pkg/labels"
)

// Config contains the config for stern
type Config struct {
	Namespace             string
	PodQuery              *regexp.Regexp
	Timestamps            bool
	ContainerQuery        *regexp.Regexp
	ExcludeContainerQuery *regexp.Regexp
	ContainerState        ContainerState
	Exclude               []*regexp.Regexp
	Include               []*regexp.Regexp
	Since                 time.Duration
	AllNamespaces         bool
	LabelSelector         labels.Selector
	TailLines             *int64
	Template              *template.Template
}

// Run starts the log watching
func Run(ctx context.Context, config *Config, cluster *kubernetes.Cluster) error {
	var namespace string

	if config.AllNamespaces {
		namespace = ""
	} else if config.Namespace == "" {
		return errors.New("no namespace set for tailing logs")
	}

	added, removed, err := Watch(ctx, cluster.Kubectl.CoreV1().Pods(namespace), config.PodQuery, config.ContainerQuery, config.ExcludeContainerQuery, config.ContainerState, config.LabelSelector)
	if err != nil {
		return errors.Wrap(err, "failed to set up watch")
	}

	tails := make(map[string]*Tail)

	go func() {
		for p := range added {
			id := p.GetID()
			if tails[id] != nil {
				continue
			}

			tail := NewTail(p.Namespace, p.Pod, p.Container, config.Template, &TailOptions{
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

	return nil
}
