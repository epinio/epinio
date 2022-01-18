package proc

import (
	"os"
	"os/exec"

	"github.com/epinio/epinio/helpers"
)

func Get(dir, command string, arg ...string) (*exec.Cmd, error) {
	var err error

	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return nil, err
		}
	}

	p := exec.Command(command, arg...)
	p.Dir = dir

	return p, nil
}

// RunW runs the command in the current working dir
func RunW(cmd string, args ...string) (string, error) {
	return Run("", false, cmd, args...)
}

func Run(dir string, toStdout bool, cmd string, arg ...string) (string, error) {
	return helpers.RunProc(dir, toStdout, cmd, arg...)
}
