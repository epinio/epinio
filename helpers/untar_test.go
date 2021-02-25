package helpers_test

import (
	"io/ioutil"
	"os"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/suse/carrier/helpers"
)

var _ = Describe("Untar", func() {
	var targetDirectory, tarFile string
	var err error

	BeforeEach(func() {
		targetDirectory, err = ioutil.TempDir("", "carrier-test")
		Expect(err).ToNot(HaveOccurred())
		tarFile = FixturePath("untar.tgz")
	})

	AfterEach(func() {
		os.Remove(targetDirectory)
	})

	It("untar the file to the specified directory", func() {
		err := Untar(tarFile, targetDirectory)
		Expect(err).ToNot(HaveOccurred())

		expectedFile1Path := path.Join(targetDirectory, "file_1")
		expectedFile2Path := path.Join(targetDirectory, "file_2")

		Expect(expectedFile1Path).To(BeARegularFile())
		Expect(expectedFile2Path).To(BeARegularFile())

		contents, err := ioutil.ReadFile(expectedFile1Path)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(contents)).To(Equal("These are the contents of file 1\n"))

		contents, err = ioutil.ReadFile(expectedFile2Path)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(contents)).To(Equal("There are the contents of file 2\n"))
	})
})
