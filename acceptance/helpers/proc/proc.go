package proc

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

func Get(dir, command string, arg ...string) (*exec.Cmd, error) {
	var commandDir string
	var err error

	if dir == "" {
		commandDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else {
		commandDir = dir
	}

	p := exec.Command(command, arg...)
	p.Dir = commandDir

	return p, nil
}

// RunW runs the command in the current working dir
func RunW(cmd string, args ...string) (string, error) {
	return Run("", false, cmd, args...)
}

func Run(dir string, toStdout bool, cmd string, arg ...string) (string, error) {
	p, err := Get(dir, cmd, arg...)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	if toStdout {
		p.Stdout = io.MultiWriter(os.Stdout, &b)
		p.Stderr = io.MultiWriter(os.Stderr, &b)
	} else {
		p.Stdout = &b
		p.Stderr = &b
	}

	if err := p.Run(); err != nil {
		return b.String(), err
	}

	err = p.Wait()
	return b.String(), err
}
