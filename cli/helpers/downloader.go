package helpers

import (
	"io"
	"net/http"
	"os"
	"path"
)

// DownloadFile downloads the given url as a file with "name" under "directory"
func DownloadFile(url, name, directory string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(path.Join(directory, name))
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
