package helpers

import (
	"bytes"
	"fmt"
	"io"
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
