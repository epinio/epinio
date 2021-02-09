package acceptance_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/codeskyblue/kexec"
	"github.com/onsi/ginkgo/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

var nodeSuffix, nodeTmpDir string

var _ = SynchronizedBeforeSuite(func() []byte {
	buildCarrier()
	return []byte(strconv.Itoa(int(time.Now().Unix())))
}, func(randomSuffix []byte) {
	var err error

	nodeSuffix = fmt.Sprintf("%d-%s",
		config.GinkgoConfig.ParallelNode, string(randomSuffix))
	nodeTmpDir, err = ioutil.TempDir("", "carrier-"+nodeSuffix)
	if err != nil {
		panic("Could not create temp dir: " + err.Error())
	}

	copyCarrier()
	createCluster()
	os.Setenv("KUBECONFIG", nodeTmpDir+"/kubeconfig")
	installCarrier()
})

var _ = AfterSuite(func() {
	deleteCluster()
	deleteTmpDir()
})

func createCluster() {
	name := fmt.Sprintf("carrier-acceptance-%s", nodeSuffix)

	if _, err := exec.LookPath("k3d"); err != nil {
		panic("Couldn't find k3d in PATH: " + err.Error())
	}

	_, err := RunProc("k3d cluster create "+name, nodeTmpDir, false)
	if err != nil {
		panic("Creating k3d cluster failed: " + err.Error())
	}

	kubeconfig, err := RunProc("k3d kubeconfig get "+name, nodeTmpDir, false)
	if err != nil {
		panic("Getting kubeconfig failed: " + err.Error())
	}
	err = ioutil.WriteFile(path.Join(nodeTmpDir, "kubeconfig"), []byte(kubeconfig), 0644)
	if err != nil {
		panic("Writing kubeconfig failed: " + err.Error())
	}
}

func deleteCluster() {
	name := fmt.Sprintf("carrier-acceptance-%s", nodeSuffix)

	if _, err := exec.LookPath("k3d"); err != nil {
		panic("Couldn't find k3d in PATH: " + err.Error())
	}

	output, err := RunProc("k3d cluster delete "+name, nodeTmpDir, false)
	if err != nil {
		panic(fmt.Sprintf("Deleting k3d cluster failed: %s\n %s\n",
			output, err.Error()))
	}
}

func deleteTmpDir() {
	err := os.RemoveAll(nodeTmpDir)
	if err != nil {
		panic(fmt.Sprintf("Failed deleting temp dir %s: %s\n",
			nodeTmpDir, err.Error()))
	}
}

func RunProc(cmd, dir string, toStdout bool) (string, error) {
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

func buildCarrier() {
	output, err := RunProc("make", "", false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Carrier: %s\n %s\n"+err.Error(), output))
	}
}

func copyCarrier() {
	output, err := RunProc("cp dist/carrier-* "+nodeTmpDir+"/carrier", "", false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't copy Carrier: %s\n %s\n"+err.Error(), output))
	}
}

func installCarrier() (string, error) {
	return Carrier("install", "")
}

// Carrier invoces the `carrier` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It default to the current dir if left empty.
func Carrier(command string, dir string) (string, error) {
	var commandDir string
	var err error

	if dir == "" {
		commandDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	} else {
		commandDir = dir
	}

	cmd := fmt.Sprintf(nodeTmpDir+"/carrier %s", command)

	return RunProc(cmd, commandDir, false)
}
