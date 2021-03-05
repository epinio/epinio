package tailer

import (
	"context"
	"regexp"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/suse/carrier/kubernetes"
	"github.com/suse/carrier/paas/ui"
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

// Notes on the above:
//   - For containers `ContainerQuery` is applied before
//     `ExcludeContainerQuery`. IOW use `CQ` to get an initial list of
//     containers and then use `ECQ` to pare that down further.
//
//   - For log entries `Exclude` is applied before `Include`.

// Run starts the log watching
func Run(ui *ui.UI, ctx context.Context, config *Config, cluster *kubernetes.Cluster) error {
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

			tail := NewTail(ui, p.Namespace, p.Pod, p.Container, config.Template, &TailOptions{
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
