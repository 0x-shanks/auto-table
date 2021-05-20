package pkg

import (
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/hourglasshoro/auto-table/pkg/migration"
	"github.com/hourglasshoro/auto-table/pkg/sql"
	"github.com/spf13/afero"
	"io/fs"
)

type Converter struct {
	Dialect    dialect.Dialect
	AutoID     bool // Flag to automatically set id as primary key
	SourceDir  string
	OutputDir  string
	FileSystem *afero.Fs
	Marker     string
}

func NewConverter(
	sourceDir string,
	outputDir string,
	fileSystem *afero.Fs,
	marker string,
) *Converter {
	return &Converter{
		Dialect:    dialect.NewMySQL(),
		AutoID:     true,
		SourceDir:  sourceDir,
		OutputDir:  outputDir,
		FileSystem: fileSystem,
		Marker:     fmt.Sprintf("+%s", marker),
	}
}

func (c *Converter) CreateSQL() (err error) {
	var filenames []string
	err = afero.Walk(*c.FileSystem, c.SourceDir,
		func(path string, info fs.FileInfo, err error) error {
			if !info.IsDir() {
				filenames = append(filenames, path)
			}
			return nil
		},
	)
	if err != nil {
		return err
	}

	sqls, dependencyMap, err := sql.CreateSQL(c.Dialect, c.AutoID, c.Marker, filenames)
	m := migration.NewMigrate(sqls, dependencyMap, c.OutputDir)
	err = m.Output(c.FileSystem)
	return
}
