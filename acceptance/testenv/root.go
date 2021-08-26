package testenv

import "path"

var root = ".."

func SetRoot(dir string) {
	root = dir
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
