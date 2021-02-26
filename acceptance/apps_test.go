package acceptance_test

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", func() {
	var org = "apps-org"
	BeforeEach(func() {
		out, err := Carrier("create-org "+org, "")
		Expect(err).ToNot(HaveOccurred(), out)
		out, err = Carrier("target "+org, "")
		Expect(err).ToNot(HaveOccurred(), out)
	})
	Describe("push and delete", func() {
		var appName string
		BeforeEach(func() {
			appName = "apps-" + strconv.Itoa(int(time.Now().Nanosecond()))
		})

		It("pushes and deletes an app", func() {
			By("pushing the app")
			currentDir, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())
			appDir := path.Join(currentDir, "../sample-app")

			out, err := Carrier(fmt.Sprintf("push %s --verbosity 1", appName), appDir)
			Expect(err).ToNot(HaveOccurred(), out)
			out, err = Carrier("apps", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(appName + `.*\|.*1\/1.*\|.*`))

			By("deleting the app")
			out, err = Carrier("delete "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)
			// TODO: Fix `carrier delete` from returning before the app is deleted #131
			Eventually(func() string {
				out, err := Carrier("apps", "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))
		})
	})
})
