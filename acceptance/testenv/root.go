package testenv

var root = ".."

func SetRoot(dir string) {
	root = dir
}

func Root() string {
	return root
}
