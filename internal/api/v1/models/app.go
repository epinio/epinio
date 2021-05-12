package models

import "fmt"

const (
	EpinioImageIDLabel = "epinio.suse.org/image-id"
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

// GitURL returns the git URL by combining the server with the org and name
func (a *App) GitURL(server string) string {
	return fmt.Sprintf("%s/%s/%s", server, a.Org, a.Name)
}

// ImageURL returns the URL of the image, using the ImageID. The ImageURL is
// later used in app.yml.  Since the final commit is not know when the app.yml
// is written, we cannot use Repo.Revision
func (a *App) ImageURL(server string) string {
	return fmt.Sprintf("%s/%s-%s", server, a.Name, a.Image.ID)
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
