package models

import (
	"context"
	"fmt"
	"regexp"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/duration"
	"github.com/pkg/errors"
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

func (app *App) StagingLogs(ctx context.Context, client *kubernetes.Cluster, follow bool, stageID string) (chan tailer.ContainerLogLine, error) {
	return app.Logs(ctx, client, follow, stageID)
}

func (app *App) RuntimeLogs(ctx context.Context, client *kubernetes.Cluster, follow bool) (chan tailer.ContainerLogLine, error) {
	return app.Logs(ctx, client, follow, "")
}

// Logs returns a channel of tailer.ContainerLogLine . Consumers can decide what
// to do with it (print it, send it as json etc).
// When stageID is an empty string, no staging logs are returned. If it is seti,
// then only logs from that staging process are returned.
func (app *App) Logs(ctx context.Context, client *kubernetes.Cluster, follow bool, stageID string) (chan tailer.ContainerLogLine, error) {
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
			{"app.kubernetes.io/managed-by", "epinio"},
			{EpinioStageIDLabel, stageID},
			{"app.kubernetes.io/part-of", app.Org},
			{"app.kubernetes.io/name", app.Name},
		}
	}

	for _, req := range selectors {
		req, err := labels.NewRequirement(req[0], selection.Equals, []string{req[1]})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*req)
	}

	logChan, err := tailer.Run(ctx, &tailer.Config{
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
	}, client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start log tail")
	}

	return logChan, nil
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
