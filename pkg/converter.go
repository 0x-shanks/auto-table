package pkg

import (
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"go/ast"
	"log"
)

const (
	commentPrefix       = "//"
	marker              = "+test"
	annotationSeparator = ':'
)

type Converter struct {
	dialect dialect.Dialect
}

func NewConverter() *Converter {
	return &Converter{
		dialect: dialect.NewMySQL(),
	}
}

func (c *Converter) CreateTable() error {
	filenames := []string{"./example/domain/micropost.go", "./example/domain/user.go", "./example/domain/birthday.go"}

	structASTMap := make(map[string]*structAST)

	for _, filename := range filenames {
		m, err := makeStructASTMap(filename)
		if err != nil {
			log.Fatal(err)
			return err
		}
		for k, v := range m {
			structASTMap[k] = v
		}
	}

	structMap := map[string]*table{}
	for name, structAST := range structASTMap {
		for _, fld := range structAST.StructType.Fields.List {
			typeName, err := detectTypeName(fld)
			if err != nil {
				log.Fatal(err)
				return err
			}
			f, err := newField(c.dialect, name, typeName, fld)
			if err != nil {
				return err
			}
			if f.Ignore {
				continue
			}
			if !(ast.IsExported(f.Name) || (f.Name == "_" && f.Name != f.Column)) {
				continue
			}
			if structMap[name] == nil {
				structMap[name] = &table{
					Option: structAST.Annotation.Option,
				}
			}
			structMap[name].Fields = append(structMap[name].Fields, f)
		}
	}

	// struct names
	names := make([]string, 0, len(structMap))
	for name := range structMap {
		names = append(names, name)
	}

	var schemas []string
	for _, name := range names {
		tbl := structMap[name]
		fields := make([]dialect.Field, len(tbl.Fields))
		for i, f := range tbl.Fields {
			fields[i] = f.ToField()
		}
		newPks := makePrimaryKeyColumns(tbl.Fields)
		pkColumns := make([]string, len(newPks))
		for i, pk := range newPks {
			pkColumns[i] = pk.ToField().Name
		}
		schemas = append(schemas, c.dialect.CreateTableSQL(dialect.Table{
			Name:        name,
			Fields:      fields,
			PrimaryKeys: pkColumns,
			Option:      tbl.Option,
		})...)
	}

	log.Println(schemas)

	return nil
}
