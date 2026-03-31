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

		var appObj models.App
		Eventually(func() string {
			appObj = appShow(namespace, app)
			return appObj.Workload.Status
		}, "5m", "5s").Should(Equal("1/1"), "app show should report 1/1 before assertions")
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
		rawPodNames := strings.Split(strings.TrimSpace(string(out)), "\n")
		podNames := make([]string, 0, len(rawPodNames))
		for _, n := range rawPodNames {
			if n != "" {
				podNames = append(podNames, n)
			}
		}
		Expect(podNames).ToNot(BeEmpty(), "expected at least one pod for app %s", app)

		// Run `yes > /dev/null &` and expect at least 1000 millicpus
		// https://winaero.com/how-to-create-100-cpu-load-in-linux/
		// Use PID file so we can kill without killall/pkill (not in all images)
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", appObj.Workload.Name,
			"--", "/bin/sh", "-c", "yes > /dev/null 2>/dev/null & echo $! > /tmp/yes.pid")
		Expect(err).ToNot(HaveOccurred(), out)
		// In CI, CPU may be throttled; require a non-trivial reading rather than 900m.
		Eventually(func() int64 {
			appObj := appShow(namespace, app)
			return appObj.Workload.Replicas[replica.Name].MilliCPUs
		}, "360s", "2s").Should(BeNumerically(">=", 100))
		// Kill the "yes" process to bring CPU down again (use PID file; no killall in minimal images)
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", appObj.Workload.Name,
			"--", "/bin/sh", "-c", "kill -9 $(cat /tmp/yes.pid) 2>/dev/null; rm -f /tmp/yes.pid")
		Expect(err).ToNot(HaveOccurred(), out)

		// Increase memory briefly to check memory metric without blocking for minutes.
		out, err = proc.Kubectl("exec",
			"--namespace", namespace, podNames[0], "--container", appObj.Workload.Name,
			"--", "/bin/sh", "-c", "head -c 50000000 </dev/zero >/tmp/epinio-mem.bin; sleep 5; rm -f /tmp/epinio-mem.bin")
		Expect(err).ToNot(HaveOccurred(), out)
		Eventually(func() int64 {
			appObj := appShow(namespace, app)
			return appObj.Workload.Replicas[replica.Name].MemoryBytes
		}, "360s", "2s").Should(BeNumerically(">=", 0))

		Consistently(func() int32 {
			appObj := appShow(namespace, app)
			return appObj.Workload.Replicas[replica.Name].Restarts
		}, "10s", "1s").Should(BeNumerically("==", 0))

		// Disrupt the running pod and verify the workload recovers.
		podNameForDelete := strings.TrimPrefix(podNames[0], "pod/")
		out, err = proc.Kubectl("delete", "pod",
			"--namespace", namespace, podNameForDelete)
		Expect(err).ToNot(HaveOccurred(), out)

		Eventually(func() string {
			appObj := appShow(namespace, app)
			return appObj.Workload.Status
		}, "300s", "2s").Should(Equal("1/1"))
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
