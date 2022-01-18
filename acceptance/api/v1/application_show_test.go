package v1_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppShow Endpoint", func() {
	var (
		namespace string
	)
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})
	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("lists the application data", func() {
		app := catalog.NewAppName()
		env.MakeContainerImageApp(app, 1, containerImageURL)
		defer env.DeleteApp(app)

		appObj := appFromAPI(namespace, app)
		Expect(appObj.Workload.Status).To(Equal("1/1"))
		createdAt, err := time.Parse(time.RFC3339, appObj.Workload.CreatedAt)
		Expect(err).ToNot(HaveOccurred())
		Expect(createdAt.Unix()).To(BeNumerically("<", time.Now().Unix()))

		Expect(appObj.Workload.Restarts).To(BeNumerically("==", 0))

		Expect(appObj.Workload.DesiredReplicas).To(BeNumerically("==", 1))
		Expect(appObj.Workload.ReadyReplicas).To(BeNumerically("==", 1))

		out, err := proc.Kubectl("get", "pods",
			fmt.Sprintf("--selector=app.kubernetes.io/name=%s", app),
			"--namespace", namespace, "--output", "name")
		Expect(err).ToNot(HaveOccurred())
		podNames := strings.Split(string(out), "\n")

		// Run `yes > /dev/null &` and expect at least 1000 millicpus
		// https://winaero.com/how-to-create-100-cpu-load-in-linux/
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", app,
			"--", "bin/sh", "-c", "yes > /dev/null 2> /dev/null &")
		Expect(err).ToNot(HaveOccurred(), out)
		Eventually(func() int64 {
			appObj := appFromAPI(namespace, app)
			return appObj.Workload.MilliCPUs
		}, "240s", "1s").Should(BeNumerically(">=", 900))
		// Kill the "yes" process to bring CPU down again
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", app,
			"--", "killall", "-9", "yes")
		Expect(err).ToNot(HaveOccurred(), out)

		// Increase memory for 3 minutes to check memory metric
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", app,
			"--", "bin/bash", "-c", "cat <( </dev/zero head -c 50m) <(sleep 180) | tail")
		Expect(err).ToNot(HaveOccurred(), out)
		Eventually(func() int64 {
			appObj := appFromAPI(namespace, app)
			return appObj.Workload.MemoryBytes
		}, "240s", "1s").Should(BeNumerically(">=", 0))

		// Kill a linkerd proxy container and see the count staying unchanged
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", "linkerd-proxy",
			"--", "bin/sh", "-c", "kill 1")
		Expect(err).ToNot(HaveOccurred(), out)

		Consistently(func() int32 {
			appObj := appFromAPI(namespace, app)
			return appObj.Workload.Restarts
		}, "5s", "1s").Should(BeNumerically("==", 0))

		// Kill an app container and see the count increasing
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", app,
			"--", "bin/sh", "-c", "kill 1")
		Expect(err).ToNot(HaveOccurred(), out)

		Eventually(func() int32 {
			appObj := appFromAPI(namespace, app)
			return appObj.Workload.Restarts
		}, "4s", "1s").Should(BeNumerically("==", 1))
	})

	It("returns a 404 when the namespace does not exist", func() {
		app := catalog.NewAppName()
		env.MakeContainerImageApp(app, 1, containerImageURL)
		defer env.DeleteApp(app)

		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/applications/%s",
			serverURL, v1.Root, app), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 404 when the app does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/bogus",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
