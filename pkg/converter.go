package pkg

import (
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/naoina/go-stringutil"
	"go/ast"
	"log"
	"strings"
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

			var fieldName *string
			var foreignKey *dialect.ForeignKey

			// Check to see if it is a dependent struct
			if parent, ok := structASTMap[strings.ToLower(typeName)]; ok {
				var hasID bool
				for _, f := range parent.StructType.Fields.List {
					idCandidate := "id"
					if strings.ToLower(f.Names[0].Name) == idCandidate {
						hasID = true
						typeName, err = detectTypeName(f.Type)
						f := fmt.Sprintf("%v%v", stringutil.ToUpperCamelCase(fld.Names[0].Name), stringutil.ToUpperCamelCase(idCandidate))
						fieldName = &f
						foreignKey = &dialect.ForeignKey{
							Table:  parent.Name,
							Column: idCandidate,
						}
					}
				}
				if !hasID {
				}
			}
			if err != nil {
				log.Fatal(err)
				return err
			}
			f, err := newField(c.dialect, name, typeName, fieldName, fld, foreignKey)
			if err != nil {
				return err
			}
			if f.Ignore {
				continue
			}
			if !(ast.IsExported(f.Name) || (f.Name == "_" && f.Name != f.Column)) {
				log.Println(f.Name)
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
		pks := makePrimaryKeyColumns(tbl.Fields)
		pkColumns := make([]string, len(pks))
		for i, pk := range pks {
			pkColumns[i] = pk.ToField().Name
		}
		fksColumns := makeForeignKeyColumns(tbl.Fields)
		schemas = append(schemas, c.dialect.CreateTableSQL(dialect.Table{
			Name:        name,
			Fields:      fields,
			PrimaryKeys: pkColumns,
			ForeignKeys: fksColumns,
			Option:      tbl.Option,
		})...)
	}

	log.Println(schemas)

	return nil
}
