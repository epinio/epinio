package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/template"

	"github.com/pkg/errors"
)

func goCopy(wait *sync.WaitGroup, dst io.WriteCloser, src io.Reader) {
	wait.Add(1)
	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			fmt.Println(err)
		}
		dst.Close()
		wait.Done()
	}()
}

func goCopyBuffer(wait *sync.WaitGroup, dst io.Writer, src io.Reader) {
	wait.Add(1)
	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			fmt.Println(err)
		}
		wait.Done()
	}()
}

// ExecTemplate renders a template and then runs Exec for it
func ExecTemplate(tmpl string, data interface{}) (string, error) {
	bashTemplate, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse template for shell script")
	}

	var script bytes.Buffer
	err = bashTemplate.Execute(&script, data)
	if err != nil {
		return "", errors.Wrap(err, "failed to render bash template")
	}

	return Exec(script)
}

// Exec runs a bash script
func Exec(script bytes.Buffer) (string, error) {
	bash := exec.Command("bash")
	var stdin io.WriteCloser
	var stdout, stderr io.ReadCloser
	var err error

	if stdin, err = bash.StdinPipe(); err != nil {
		return "", errors.Wrap(err, "failed to get stdin pipe for bash process")
	}
	if stdout, err = bash.StdoutPipe(); err != nil {
		return "", errors.Wrap(err, "failed to get stdout pipe for bash process")
	}
	if stderr, err = bash.StderrPipe(); err != nil {
		return "", errors.Wrap(err, "failed to get stderr pipe for bash process")
	}

	wait := sync.WaitGroup{}

	buffer := new(strings.Builder)

	goCopy(&wait, stdin, &script)
	goCopyBuffer(&wait, buffer, stdout)
	goCopy(&wait, os.Stderr, stderr)

	if err := bash.Start(); err != nil {
		return "", errors.Wrap(err, "failed to start bash process")
	}

	wait.Wait()

	if err := bash.Wait(); err != nil {
		return "", errors.Wrap(err, "script execution failed")
	}

	return buffer.String(), nil
}
