package helpers_test

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"

	. "github.com/epinio/epinio/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DownloadFile", func() {
	var url string
	var sourceDirectory, targetDirectory string
	var err error

	BeforeEach(func() {
		sourceDirectory, err = ioutil.TempDir("", "epinio-test")

		file, err := os.Create(path.Join(sourceDirectory, "thefile"))
		Expect(err).ToNot(HaveOccurred())
		defer file.Close()

		file.Write([]byte("the file contents"))

		dirURL, err := ServeDir(sourceDirectory)
		Expect(err).ToNot(HaveOccurred())

		url = "http://" + dirURL + "/thefile"
	})

	It("downloads a url with filename under directory", func() {
		targetDirectory, err = ioutil.TempDir("", "epinio-test")

		err = DownloadFile(url, "downloadedFile", targetDirectory)
		Expect(err).ToNot(HaveOccurred())
		targetPath := path.Join(targetDirectory, "downloadedFile")
		defer os.Remove(targetPath)

		Expect(targetPath).To(BeARegularFile())

		contents, err := ioutil.ReadFile(targetPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(contents)).To(Equal("the file contents"))
	})

	AfterEach(func() {
		os.RemoveAll(sourceDirectory)
		os.RemoveAll(targetDirectory)
	})
})

// ServeDir serves the directory on a random port over http and returns the url
// where it can be reached.
func ServeDir(directory string) (string, error) {
	http.Handle("/", http.FileServer(http.Dir(directory)))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go func() {
		panic(http.Serve(listener, nil))
	}()

	return listener.Addr().String(), nil
}
