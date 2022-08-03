package file

type Backend struct {
	DirPath string
}

type fileItrWrapper struct {
	DirPath string
}

type fileWrapper struct {
	FileName string
	Path     string
}

type file struct {
	Path string
}
