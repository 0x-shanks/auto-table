package pkg

import (
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/hourglasshoro/auto-table/pkg/sql"
)

type Generator struct {
	Dialect  dialect.Dialect
	AutoID   bool // Flag to automatically set id as primary key
	Marker   string
	TagMaker string
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
	sqlMap, _, err = sql.CreateSQL(g.Dialect, g.AutoID, g.Marker, g.TagMaker, filenames)
	return
}
