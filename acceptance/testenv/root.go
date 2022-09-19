package testenv

import (
	"path"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
)

var root = ".."

func SetRoot(dir string) {
	root, _ = filepath.Abs(dir)
	By("Root: " + root)
}

func Root() string {
	return root
}

// AssetPath returns the path to an asset
func AssetPath(p ...string) string {
	parts := append([]string{root, "assets"}, p...)
	return path.Join(parts...)
}

// TestAssetPath returns the relative path to the test assets
func TestAssetPath(file string) string {
	return path.Join(root, "assets", "tests", file)
}
