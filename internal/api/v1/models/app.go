package models

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/duration"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	EpinioStageIDLabel = "epinio.suse.org/stage-id"
)

// App has all the app properties, like the image, repo, route and staging information
type App struct {
	AppRef
	Image     ImageRef
	Git       *GitRef
	Route     string
	Stage     StageRef
	Instances int32
}

// NewApp returns a new app for name and org
func NewApp(name string, org string) *App {
	return &App{AppRef: AppRef{Name: name, Org: org}}
}

// Logs method writes log lines to the specified logChan. The caller can stop
// the logging with the ctx cancelFunc. It's also the callers responsibility
// to close the logChan when done.
// When stageID is an empty string, no staging logs are returned. If it is set,
// then only logs from that staging process are returned.
// TODO: Fix this, when stageID is set, it also returns application logs. Needs extra label?
func (app *App) Logs(ctx context.Context, logChan chan tailer.ContainerLogLine, wg *sync.WaitGroup, client *kubernetes.Cluster, follow bool, stageID string) error {
	selector := labels.NewSelector()

	var selectors [][]string
	if stageID == "" {
		selectors = [][]string{
			{"app.kubernetes.io/component", "application"},
			{"app.kubernetes.io/managed-by", "epinio"},
			{"app.kubernetes.io/part-of", app.Org},
			{"app.kubernetes.io/name", app.Name},
		}
	} else {
		selectors = [][]string{
			{"app.kubernetes.io/component", "staging"},
			{"app.kubernetes.io/managed-by", "epinio"},
			{EpinioStageIDLabel, stageID},
			{"app.kubernetes.io/part-of", app.Org},
			{"app.kubernetes.io/name", app.Name},
		}
	}

	for _, req := range selectors {
		req, err := labels.NewRequirement(req[0], selection.Equals, []string{req[1]})
		if err != nil {
			return err
		}
		selector = selector.Add(*req)
	}

	config := &tailer.Config{
		ContainerQuery:        regexp.MustCompile(".*"),
		ExcludeContainerQuery: nil,
		ContainerState:        "running",
		Exclude:               nil,
		Include:               nil,
		Timestamps:            false,
		Since:                 duration.LogHistory(),
		AllNamespaces:         true,
		LabelSelector:         selector,
		TailLines:             nil,
		Namespace:             "",
		PodQuery:              regexp.MustCompile(".*"),
	}

	if follow {
		return tailer.StreamLogs(ctx, logChan, wg, config, client)
	}

	return tailer.FetchLogs(ctx, logChan, wg, config, client)
}

// GitURL returns the git URL by combining the server with the org and name
func (app *App) GitURL(server string) string {
	return fmt.Sprintf("%s/%s/%s", server, app.Org, app.Name)
}

// ImageURL returns the URL of the image, using the ImageID. The ImageURL is
// later used in app.yml.  Since the final commit is not known when the app.yml
// is written, we cannot use Repo.Revision
func (app *App) ImageURL(server string) string {
	return fmt.Sprintf("%s/%s-%s", server, app.Name, app.Git.Revision)
}

// AppRef references an App by name and org
type AppRef struct {
	Name string `json:"name"`
	Org  string `json:"org"`
}

// StageRef references a tekton staging run by ID, currently randomly generated
// for each POST to the staging endpoint
type StageRef struct {
	ID string `json:"id,omitempty"`
}

// NewStage returns a new reference to a staging run
func NewStage(id string) StageRef {
	return StageRef{id}
}

// ImageRef references an upload
type ImageRef struct {
	ID string `json:"id,omitempty"`
}

// NewImage returns a new image ref for the given ID
func NewImage(id string) ImageRef {
	return ImageRef{id}
}

// GitRef describes a git commit in a repo
type GitRef struct {
	Revision string `json:"revision"`
	URL      string `json:"url"`
}
