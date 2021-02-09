package acceptance_test

import (
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
		_, err := Carrier("create-org "+org, "")
		Expect(err).ToNot(HaveOccurred())
		_, err = Carrier("target "+org, "")
		Expect(err).ToNot(HaveOccurred())
	})
	Describe("push", func() {
		var appName string
		BeforeEach(func() {
			appName = "apps-" + strconv.Itoa(int(time.Now().Nanosecond()))
		})
		It("pushes an app successfully", func() {
			// TODO: Fix the path when we move to the root dir
			currentDir, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())
			appDir := path.Join(currentDir, "sample-app")

			_, err = Carrier("push "+appName, appDir)
			Expect(err).ToNot(HaveOccurred())
			out, err := Carrier("apps", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchRegexp(appName + ".*|.*1/1.*|.*"))
		})
	})
})
