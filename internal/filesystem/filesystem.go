// Package filesystem allows enables the use of either embedded assets
// (with statik) or files from the real filesystem. This is useful to
// be able to develop the app using files on the disk before we ship
// it as a single binary.
package filesystem

import (
	"net/http"

	// We need the imports for the side effects, i.e. the `init()`
	// functions they run to register their filesystems. As none
	// of the public symbols are directly used an non-blank import
	// would be elided by go format.
	_ "github.com/epinio/epinio/assets/statikWebAssets"
	_ "github.com/epinio/epinio/assets/statikWebViews"
	"github.com/rakyll/statik/fs"
)

// Views returns a filesystem providing access to the web view assets.
func Views() http.FileSystem {
	fs, err := fs.NewWithNamespace("webViews")
	if err != nil {
		panic(err)
	}

	return fs
}

// Assets returns a filesystem providing access to the general web
// assets.
func Assets() http.FileSystem {
	fs, err := fs.NewWithNamespace("webAssets")
	if err != nil {
		panic(err)
	}

	return fs
}
