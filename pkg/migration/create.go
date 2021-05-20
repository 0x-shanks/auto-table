package migration

import (
	"fmt"
	sql "github.com/hourglasshoro/auto-table/pkg/sql"
	"github.com/spf13/afero"
	"time"
)

type MigrateElm struct {
	File string
	SQL  string
}

type Migrate struct {
	Up   *MigrateElm
	Down *MigrateElm
}

type Migrates struct {
	Map       map[string]*Migrate // map[tableName]Migrate
	Order     []string            // []tableName
	OutputDir string
}

func NewMigrate(sqls map[string]*sql.SQL, dependencyMap map[string]map[string]struct{}, output string) *Migrates {
	migrates := new(Migrates)
	migrates.Map = map[string]*Migrate{}
	migrates.Order = []string{}
	migrates.OutputDir = output
	t := time.Now()
	var deleteList []string
	for len(dependencyMap) > 0 {
		for tableName, dependency := range dependencyMap {
			if len(dependency) == 0 {
				migrates.Order = append(migrates.Order, tableName)
				timestamp := t.Unix()
				t = t.Add(time.Second * 1)
				migrates.Map[tableName] = &Migrate{
					Up: &MigrateElm{
						File: fmt.Sprintf("%d_add_%s_table.up.sql", timestamp, tableName),
						SQL:  sqls[tableName].Table.Create,
					},
					Down: &MigrateElm{
						File: fmt.Sprintf("%d_add_%s_table.down.sql", timestamp, tableName),
						SQL:  sqls[tableName].Table.Drop,
					},
				}
				deleteList = append(deleteList, tableName)
				delete(dependencyMap, tableName)
			}
		}

		for _, dependency := range dependencyMap {
			for _, d := range deleteList {
				delete(dependency, d)
			}
		}
	}
	return migrates
}

func (m *Migrates) Output(fs *afero.Fs) (err error) {
	for _, filename := range m.Order {

		// Up
		output := fmt.Sprintf("%s/%s", m.OutputDir, m.Map[filename].Up.File)
		wErr := afero.WriteFile(*fs, output, []byte(m.Map[filename].Up.SQL), 0644)
		if wErr != nil {
			err = wErr
			return
		}

		// Down
		output = fmt.Sprintf("%s/%s", m.OutputDir, m.Map[filename].Down.File)
		wErr = afero.WriteFile(*fs, output, []byte(m.Map[filename].Down.SQL), 0644)
		if wErr != nil {
			err = wErr
			return
		}
	}
	return
}
