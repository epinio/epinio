package helpers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/codeskyblue/kexec"
	"github.com/pkg/errors"

	"github.com/suse/carrier/paas/ui"
)

type ExternalFuncWithString func() (output string, err error)

type ExternalFunc func() (err error)

func RunProc(cmd, dir string, toStdout bool) (string, error) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Println("Executing ", cmd)
	}
	p := kexec.CommandString(cmd)

	var b bytes.Buffer
	if toStdout {
		p.Stdout = io.MultiWriter(os.Stdout, &b)
		p.Stderr = io.MultiWriter(os.Stderr, &b)
	} else {
		p.Stdout = &b
		p.Stderr = &b
	}

	p.Dir = dir

	if err := p.Run(); err != nil {
		return b.String(), err
	}

	err := p.Wait()
	return b.String(), err
}

func RunProcNoErr(cmd, dir string, toStdout bool) (string, error) {
	if os.Getenv("DEBUG") == "true" {
		fmt.Println("Executing ", cmd)
	}
	p := kexec.CommandString(cmd)

	var b bytes.Buffer
	if toStdout {
		p.Stdout = io.MultiWriter(os.Stdout, &b)
		p.Stderr = nil
	} else {
		p.Stdout = &b
		p.Stderr = nil
	}

	p.Dir = dir

	if err := p.Run(); err != nil {
		return b.String(), err
	}

	err := p.Wait()
	return b.String(), err
}

// CreateTmpFile creates a temporary file on the disk with the given contents
// and returns the path to it and an error if something goes wrong.
func CreateTmpFile(contents string) (string, error) {
	tmpfile, err := ioutil.TempFile("", "carrier")
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

// Kubectl invoces the `kubectl` command in PATH, running the specified command.
// It returns the command output and/or error.
func Kubectl(command string) (string, error) {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		return "", errors.Wrap(err, "kubectl not in path")
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cmd := fmt.Sprintf("kubectl " + command)

	return RunProc(cmd, currentdir, false)
}

func WaitForCommandCompletion(ui *ui.UI, message string, funk ExternalFuncWithString) (string, error) {
	s := ui.Progressf(" %s", message)
	defer s.Stop()

	return funk()
}

// ExecToSuccessWithTimeout retries the given function with stirng & error return,
// until it either succeeds of the timeout is reached. It retries every "interval" duration.
func ExecToSuccessWithTimeout(funk ExternalFuncWithString, timeout, interval time.Duration) (string, error) {
	timeoutChan := time.After(timeout)
	for {
		select {
		case <-timeoutChan:
			return "", errors.New(fmt.Sprintf("Timed out after %s", timeout.String()))
		default:
			if out, err := funk(); err != nil {
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

// OpenSSLSubjectHash return the subject_hash of the given CA certificate as
// returned by this command:
// openssl x509 -hash -noout
// https://www.openssl.org/docs/man1.0.2/man1/x509.html
// TODO: The way this function is implemented, it makes a system call to openssl
// thus making openssl a dependency. There must be a way to calculate the hash
// in Go so we don't need openssl.
func OpenSSLSubjectHash(cert string) (string, error) {
	_, err := exec.LookPath("openssl")
	if err != nil {
		return "", errors.Wrap(err, "openssl not in path")
	}

	cmd := exec.Command("openssl", "x509", "-hash", "-noout")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, cert)
	}()

	out, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(out)), err
}
