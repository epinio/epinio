package helpers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/termui"
)

type ExternalFuncWithString func() (output string, err error)

type ExternalFunc func() (err error)

func RunProc(dir string, toStdout bool, command string, args ...string) (string, error) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Printf("Executing: %s %v (in: %s)\n", command, args, dir)
	}
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

	return b.String(), cmd.Run()
}

func RunProcNoErr(dir string, toStdout bool, command string, args ...string) (string, error) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Printf("Executing %s %v\n", command, args)
	}
	cmd := exec.Command(command, args...)

	var b bytes.Buffer
	if toStdout {
		cmd.Stdout = io.MultiWriter(os.Stdout, &b)
		cmd.Stderr = nil
	} else {
		cmd.Stdout = &b
		cmd.Stderr = nil
	}

	cmd.Dir = dir

	return b.String(), cmd.Run()
}

// CreateTmpFile creates a temporary file on the disk with the given contents
// and returns the path to it and an error if something goes wrong.
func CreateTmpFile(contents string) (string, error) {
	tmpfile, err := ioutil.TempFile("", "epinio")
	if err != nil {
		return tmpfile.Name(), err
	}
	if _, err := tmpfile.Write([]byte(contents)); err != nil {
		return tmpfile.Name(), err
	}
	if err := tmpfile.Close(); err != nil {
		return tmpfile.Name(), err
	}

	return tmpfile.Name(), nil
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

	return RunProc(currentdir, false, "kubectl", command...)
}

// WaitForCommandCompletion prints progress dots until the func completes
func WaitForCommandCompletion(ui *termui.UI, message string, funk ExternalFuncWithString) (string, error) {
	s := ui.Progressf(" %s", message)
	defer s.Stop()

	return funk()
}

// ExecToSuccessWithTimeout retries the given function with string & error return,
// until it either succeeds of the timeout is reached. It retries every "interval" duration.
func ExecToSuccessWithTimeout(funk ExternalFuncWithString, log logr.Logger, timeout, interval time.Duration) (string, error) {
	timeoutChan := time.After(timeout)
	for {
		select {
		case <-timeoutChan:
			return "", errors.Errorf("Timed out after %s", timeout.String())
		default:
			if out, err := funk(); err != nil {
				log.Info(fmt.Sprintf("Retrying because of error: %s\n%s", err.Error(), out))
				time.Sleep(interval)
			} else {
				return out, nil
			}
		}
	}
}

// RunToSuccessWithTimeout retries the given function with error return,
// until it either succeeds or the timeout is reached. It retries every "interval" duration.
func RunToSuccessWithTimeout(funk ExternalFunc, timeout, interval time.Duration) error {
	timeoutChan := time.After(timeout)
	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("Timed out after %s", timeout.String())
		default:
			if err := funk(); err != nil {
				time.Sleep(interval)
			} else {
				return nil
			}
		}
	}
}
