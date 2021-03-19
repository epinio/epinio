// Package filesystem allows us to use either embeded assets (with statik) or
// files from the real filesystem. This is useful to be able to develop the app
// using files on the disk before we ship it as a single binary.
package filesystem

import (
	"io"
	"net/http"
	"os"

	"github.com/rakyll/statik/fs"
)

type Filesystem struct {
	local bool
}

func NewFilesystem(local bool) *Filesystem {
	return &Filesystem{
		local: local,
	}
}

func (f *Filesystem) Open(path string) (io.Reader, error) {
	if f.local {
		return os.Open(path)
	} else {
		statikFS, err := fs.New()
		if err != nil {
			return nil, err
		}
		return statikFS.Open(path)
	}
}

func Dir(dirPath string, local bool) http.FileSystem {
	if local {
		return http.Dir("." + dirPath)
	} else {
		dir, err := fs.NewWithNamespace("web")
		if err != nil {
			panic(err)
		}
		return dir
	}
}
