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
	Dialect  dialect.Dialect
	AutoID   bool // Flag to automatically set id as primary key
	ModelDir string
}

func NewConverter() *Converter {
	return &Converter{
		Dialect: dialect.NewMySQL(),
		AutoID:  true,
	}
}

const idCandidate = "id"

var intPrimitives = map[string]struct{}{"int8": {}, "int16": {}, "int32": {}, "int64": {}, "int": {}, "uint8": {}, "uint16": {}, "uint32": {}, "uint64": {}, "uint": {}}

func (c *Converter) CreateTable() error {
	filenames := []string{"./example/domain/micropost.go", "./example/domain/user.go", "./example/domain/tag.go"}

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

		var hasID bool    // Whether or not this struct has an ID field
		var idType string // Type of ID field

		for _, fld := range structAST.StructType.Fields.List {
			typeStr, typeName, _, isArray, err := detectTypeName(fld)

			var fieldName *string
			var foreignKey *dialect.ForeignKey
			var primaryKey bool
			var autoIncrement bool

			// Check if there is a field named id
			if strings.ToLower(fld.Names[0].Name) == idCandidate {
				if c.AutoID {
					primaryKey = true
					_, autoIncrement = intPrimitives[typeStr]
					hasID = true
					idType = typeStr
				}
			}

			// Check to see if it is a dependent struct
			if parent, ok := structASTMap[strings.ToLower(typeName)]; ok {
				if isArray {
					// many-to-many

					// Make cross reference table
					crossReference := fmt.Sprintf("%s_%s", name, strings.ToLower(typeName))
					structMap[crossReference] = &table{}
					// self field
					sFieldName := fmt.Sprintf("%s_id", name)
					if !hasID {
						fErr := fmt.Errorf("%s doesn't have %s", name, idCandidate)
						log.Printf("WARN: %s", fErr)
						continue
					}
					sForeignKey := &dialect.ForeignKey{
						Table:  name,
						Column: idCandidate,
					}
					f, fErr := newField(c.Dialect, name, idType, &sFieldName, fld, sForeignKey, true, false)
					if fErr != nil {
						log.Printf("WARN: %s", fErr)
						continue
					}
					structMap[crossReference].Fields = append(structMap[crossReference].Fields, f)

					// parent field
					var pTypeStr string
					var pFieldName *string
					var pForeignKey *dialect.ForeignKey
					for _, f := range parent.StructType.Fields.List {
						// Set foreign key
						if strings.ToLower(f.Names[0].Name) == idCandidate {

							pTypeStr, _, _, _, err = detectTypeName(f.Type)
							f := fmt.Sprintf("%v%v", stringutil.ToUpperCamelCase(fld.Names[0].Name), stringutil.ToUpperCamelCase(idCandidate))
							pFieldName = &f
							pForeignKey = &dialect.ForeignKey{
								Table:  parent.Name,
								Column: idCandidate,
							}
						}
					}
					f, fErr = newField(c.Dialect, name, pTypeStr, pFieldName, fld, pForeignKey, true, false)
					if fErr != nil {
						log.Printf("WARN: %s", fErr)
						continue
					}
					structMap[crossReference].Fields = append(structMap[crossReference].Fields, f)

					// This is a case of an cross reference table, so no columns are added.
					continue
				} else {
					// one-to-many
					for _, f := range parent.StructType.Fields.List {
						// Set foreign key
						if strings.ToLower(f.Names[0].Name) == idCandidate {
							typeStr, _, _, _, err = detectTypeName(f.Type)
							f := fmt.Sprintf("%v%v", stringutil.ToUpperCamelCase(fld.Names[0].Name), stringutil.ToUpperCamelCase(idCandidate))
							fieldName = &f
							foreignKey = &dialect.ForeignKey{
								Table:  parent.Name,
								Column: idCandidate,
							}
						}
					}
				}
			}

			if err != nil {
				log.Fatal(err)
				return err
			}
			f, err := newField(c.Dialect, name, typeStr, fieldName, fld, foreignKey, primaryKey, autoIncrement)
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
		schemas = append(schemas, c.Dialect.CreateTableSQL(dialect.Table{
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
