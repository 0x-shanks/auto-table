package pkg

import (
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/hourglasshoro/auto-table/pkg/file"
	"github.com/hourglasshoro/auto-table/pkg/migration"
	"github.com/hourglasshoro/auto-table/pkg/sql"
	"github.com/spf13/afero"
)

type Converter struct {
	Dialect    dialect.Dialect
	AutoID     bool // Flag to automatically set id as primary key
	SourceDir  string
	OutputDir  string
	FileSystem *afero.Fs
	Marker     string
	TagMaker   string
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
		TagMaker:   marker,
	}
}

func (c *Converter) CreateSQL() (err error) {
	filenames, err := file.GetFiles(c.FileSystem, c.SourceDir)
	sqlMap, dependencyMap, err := sql.CreateSQL(c.Dialect, c.AutoID, c.Marker, c.TagMaker, filenames)
	m := migration.NewMigrate(sqlMap, dependencyMap, c.OutputDir)
	err = m.WriteFile(c.FileSystem)
	return
}
