package pkg

import (
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/hourglasshoro/auto-table/pkg/migration"
	"github.com/hourglasshoro/auto-table/pkg/sql"
	"github.com/spf13/afero"
)

type Generator struct {
	Dialect       dialect.Dialect
	AutoID        bool // Flag to automatically set id as primary key
	Marker        string
	TagMaker      string
	SQLMap        map[string]*sql.SQL
	DependencyMap map[string]map[string]struct{}
}

func NewGenerator(marker string) *Generator {
	return &Generator{
		Dialect:  dialect.NewMySQL(),
		AutoID:   true,
		Marker:   fmt.Sprintf("+%s", marker),
		TagMaker: marker,
	}
}

func (g *Generator) CreateSQL(filenames []string) (sqlMap map[string]*sql.SQL, err error) {
	sqlMap, dependencyMap, err := sql.CreateSQL(g.Dialect, g.AutoID, g.Marker, g.TagMaker, filenames)
	g.SQLMap = sqlMap
	g.DependencyMap = dependencyMap
	return
}

func (g *Generator) WriteFile(fs *afero.Fs, outPutDir string, f func(content string, filename string) error) (err error) {
	// TODO: DO refactor
	m := migration.NewMigrate(g.SQLMap, g.DependencyMap, outPutDir)
	for _, filename := range m.Order {

		// Up
		output := fmt.Sprintf("%s/%s", m.OutputDir, m.Map[filename].Up.File)
		wErr := f(m.Map[filename].Up.SQL, output)
		if wErr != nil {
			err = wErr
			return
		}

		// Down
		output = fmt.Sprintf("%s/%s", m.OutputDir, m.Map[filename].Down.File)
		wErr = f(m.Map[filename].Down.SQL, output)
		if wErr != nil {
			err = wErr
			return
		}
	}
	return
}
