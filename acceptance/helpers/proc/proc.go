// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proc

import (
	"bytes"
	"io"
	"os"
	"os/exec"

	"github.com/pkg/errors"
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

func Run(dir string, toStdout bool, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	var b bytes.Buffer
	if toStdout {
		cmd.Stdout = io.MultiWriter(os.Stdout, &b)
		cmd.Stderr = io.MultiWriter(os.Stderr, &b)
	} else {
		cmd.Stdout = &b
		cmd.Stderr = &b
	}

	cmd.Dir = dir

	err := cmd.Run()
	return b.String(), err
}

// Kubectl invokes the `kubectl` command in PATH, running the specified command.
// It returns the command output and/or error.
func Kubectl(command ...string) (string, error) {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return "", errors.Wrap(err, "kubectl not in path")
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return Run(currentdir, false, "kubectl", command...)
}
