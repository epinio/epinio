package gitea

import (
	"fmt"
	"strings"

	"github.com/suse/carrier/shim/pkg/shell"
)

// PushInfo contains info required to push an app
type PushInfo struct {
	Target        string
	Username      string
	Password      string
	AppName       string
	DroneToken    string
	AppDir        string
	ImageUser     string
	ImagePassword string
}

// CreateRepo creates a repo
func CreateRepo(info PushInfo) error {
	_, err := shell.ExecTemplate(CreateRepoScript, info)
	if err != nil {
		return err
	}

	droneToken, err := shell.ExecTemplate(DroneTokenScript, info)
	if err != nil {
		return err
	}

	info.DroneToken = droneToken
	_, err = shell.ExecTemplate(EnableDroneScript, info)
	if err != nil {
		return err
	}

	return nil
}

// Push pushes app code to gitea
func Push(info PushInfo) error {
	_, err := shell.ExecTemplate(PrepareCodeScript, info)
	if err != nil {
		return err
	}

	_, err = shell.ExecTemplate(PushScript, info)
	if err != nil {
		return err
	}

	return nil
}

// StagingStatus returns the staging status of an app
func StagingStatus(info PushInfo) (string, error) {
	status, err := shell.ExecTemplate(StagingStatusScript, info)
	if err != nil {
		fmt.Println(err.Error())
		return "STAGING", nil
	}

	return strings.TrimSpace(status), nil
}
