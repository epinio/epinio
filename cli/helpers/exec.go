package helpers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/codeskyblue/kexec"
)

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
