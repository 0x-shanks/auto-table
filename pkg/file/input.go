package file

import (
	"github.com/spf13/afero"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// Solve resolves the path of the root directory to be searched from the input source and currentDir
func Solve(source string, currentDir string) (resolvedDir string) {
	if source != "" && !filepath.IsAbs(source) {
		resolvedDir = path.Join(currentDir, source)
	} else if filepath.IsAbs(source) {
		resolvedDir = source
	} else {
		resolvedDir = currentDir
	}
	return
}

func isGoFile(filename string) bool {
	pos := strings.LastIndex(filename, ".")
	if filename[pos:] == ".go" {
		return true
	}
	return false
}

func isGoTest(filename string) bool {
	f := strings.Split(filename, ".")
	pos := strings.LastIndex(f[0], "_")
	if pos == -1 {
		return false
	}
	if f[0][pos:] == "_test" {
		return true
	}
	return false
}

// GetFiles enumerates the files that exist in the specified directory
func GetFiles(fileSystem *afero.Fs, dir string) (filenames []string, err error) {
	err = afero.Walk(*fileSystem, dir,
		func(path string, info fs.FileInfo, err error) error {
			if !info.IsDir() {
				if !isGoFile(path) {
					return nil
				}
				if isGoTest(path) {
					return nil
				}
				filenames = append(filenames, path)
			}
			return nil
		},
	)
	return
}
