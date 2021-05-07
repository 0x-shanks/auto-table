package pkg

import (
	"path"
	"path/filepath"
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
