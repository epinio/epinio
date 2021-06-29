package proc

import (
	"bytes"
	"io"
	"os"

	"github.com/codeskyblue/kexec"
)

func Get(command string, dir string) (*kexec.KCommand, error) {
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

	p := kexec.CommandString(command)
	p.Dir = commandDir

	return p, nil
}

func Run(cmd, dir string, toStdout bool) (string, error) {
	p, err := Get(cmd, dir)
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
