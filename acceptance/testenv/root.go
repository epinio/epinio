package testenv

import "path"

var root = ".."

func SetRoot(dir string) {
	root = dir
}

func Root() string {
	return root
}

func AssetPath(file string) string {
	return path.Join(root, "assets", "tests", file)
}
