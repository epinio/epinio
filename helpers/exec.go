package helpers

import (
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/termui"
)

type ExternalFuncWithString func() (output string, err error)

type ExternalFunc func() (err error)

// CreateTmpFile creates a temporary file on the disk with the given contents
// and returns the path to it and an error if something goes wrong.
func CreateTmpFile(contents string) (string, error) {
	tmpfile, err := os.CreateTemp("", "epinio")
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
