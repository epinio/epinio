// Package filesystem allows us to use either embeded assets (with statik) or
// files from the real filesystem. This is useful to be able to develop the app
// using files on the disk before we ship it as a single binary.
package filesystem

import (
	"net/http"

	"github.com/rakyll/statik/fs"
	_ "github.com/suse/carrier/statikWebAssets"
	_ "github.com/suse/carrier/statikWebViews"
)

func Views() http.FileSystem {
	fs, err := fs.NewWithNamespace("webViews")
	if err != nil {
		panic(err)
	}

	return fs
}

func Assets() http.FileSystem {
	fs, err := fs.NewWithNamespace("webAssets")
	if err != nil {
		panic(err)
	}

	return fs
}
