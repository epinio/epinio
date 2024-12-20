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

package v1_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppShow Endpoint", LApplication, func() {
	var (
		namespace string
	)
	containerImageURL := "epinio/sample-app"

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

		appObj := appShow(namespace, app)
		Expect(appObj.Workload.Status).To(Equal("1/1"))
		createdAt, err := time.Parse(time.RFC3339, appObj.Workload.CreatedAt)
		Expect(err).ToNot(HaveOccurred())
		Expect(createdAt.Unix()).To(BeNumerically("<", time.Now().Unix()))

		Expect(appObj.Workload.DesiredReplicas).To(BeNumerically("==", 1))
		Expect(appObj.Workload.ReadyReplicas).To(BeNumerically("==", 1))

		Expect(len(appObj.Workload.Replicas)).To(Equal(1))
		var replica *models.PodInfo
		for _, v := range appObj.Workload.Replicas {
			replica = v
			break
		}
		Expect(replica.Restarts).To(BeNumerically("==", 0))

		out, err := proc.Kubectl("get", "pods",
			fmt.Sprintf("--selector=app.kubernetes.io/name=%s", app),
			"--namespace", namespace, "--output", "name")
		Expect(err).ToNot(HaveOccurred())
		podNames := strings.Split(string(out), "\n")

		// Run `yes > /dev/null &` and expect at least 1000 millicpus
		// https://winaero.com/how-to-create-100-cpu-load-in-linux/
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", appObj.Workload.Name,
			"--", "bin/sh", "-c", "yes > /dev/null 2> /dev/null &")
		Expect(err).ToNot(HaveOccurred(), out)
		Eventually(func() int64 {
			appObj := appShow(namespace, app)
			return appObj.Workload.Replicas[replica.Name].MilliCPUs
		}, "240s", "1s").Should(BeNumerically(">=", 900))
		// Kill the "yes" process to bring CPU down again
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", appObj.Workload.Name,
			"--", "killall", "-9", "yes")
		Expect(err).ToNot(HaveOccurred(), out)

		// Increase memory for 3 minutes to check memory metric
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", appObj.Workload.Name,
			"--", "bin/bash", "-c", "cat <( </dev/zero head -c 50m) <(sleep 180) | tail")
		Expect(err).ToNot(HaveOccurred(), out)
		Eventually(func() int64 {
			appObj := appShow(namespace, app)
			return appObj.Workload.Replicas[replica.Name].MemoryBytes
		}, "240s", "1s").Should(BeNumerically(">=", 0))

		Consistently(func() int32 {
			appObj := appShow(namespace, app)
			return appObj.Workload.Replicas[replica.Name].Restarts
		}, "10s", "1s").Should(BeNumerically("==", 0))

		// Kill an app container and see the count increasing
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", appObj.Workload.Name,
			"--", "bin/sh", "-c", "kill 1")
		Expect(err).ToNot(HaveOccurred(), out)

		Eventually(func() int32 {
			appObj := appShow(namespace, app)
			return appObj.Workload.Replicas[replica.Name].Restarts
		}, "15s", "1s").Should(BeNumerically("==", 1))
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
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})

	It("returns a 404 when the app does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications/bogus",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
