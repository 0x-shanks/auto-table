package pkg

import (
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/naoina/go-stringutil"
	"github.com/spf13/afero"
	"go/ast"
	"io/fs"
	"log"
	"strings"
	"time"
)

const (
	commentPrefix       = "//"
	marker              = "+test"
	annotationSeparator = ':'
)

type Converter struct {
	Dialect    dialect.Dialect
	AutoID     bool // Flag to automatically set id as primary key
	SourceDir  string
	OutputDir  string
	FileSystem afero.Fs
}

func NewConverter(
	sourceDir string,
	outputDir string,
	fileSystem afero.Fs,
) *Converter {
	return &Converter{
		Dialect:    dialect.NewMySQL(),
		AutoID:     true,
		SourceDir:  sourceDir,
		OutputDir:  outputDir,
		FileSystem: fileSystem,
	}
}

const idCandidate = "id"

var intPrimitives = map[string]struct{}{"int8": {}, "int16": {}, "int32": {}, "int64": {}, "int": {}, "uint8": {}, "uint16": {}, "uint32": {}, "uint64": {}, "uint": {}}

func (c *Converter) CreateSQL() error {
	var filenames []string
	err := afero.Walk(c.FileSystem, c.SourceDir,
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
	dependencyMap := map[string]map[string]struct{}{} //map[tableName]dependency

	for name, structAST := range structASTMap {

		var hasID bool    // Whether or not this struct has an ID field
		var idType string // Type of ID field
		dependencyMap[name] = map[string]struct{}{}

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
					dependencyMap[crossReference] = map[string]struct{}{}

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
					dependencyMap[crossReference][name] = struct{}{}
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
							dependencyMap[crossReference][stringutil.ToSnakeCase(parent.Name)] = struct{}{}
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
							dependencyMap[name][stringutil.ToSnakeCase(parent.Name)] = struct{}{}
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

	type schema struct {
		Create string
		Drop   string
	}

	schemas := map[string]*schema{} // map[tableName]schema
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
		t := dialect.Table{
			Name:        name,
			Fields:      fields,
			PrimaryKeys: pkColumns,
			ForeignKeys: fksColumns,
			Option:      tbl.Option,
		}
		createSchema := strings.Join(c.Dialect.CreateTableSQL(t), "")
		dropSchema := strings.Join(c.Dialect.DropTableSQL(t), "")

		schemas[name] = &schema{
			Create: createSchema,
			Drop:   dropSchema,
		}
	}

	t := time.Now()
	var deleteList []string
	for len(dependencyMap) > 0 {
		for tableName, dependency := range dependencyMap {
			if len(dependency) == 0 {
				timestamp := t.Unix()
				t = t.Add(time.Second * 1)
				filename := fmt.Sprintf("%s/%d_add_%s_table.up.sql", c.OutputDir, timestamp, tableName)
				err := afero.WriteFile(c.FileSystem, filename, []byte(schemas[tableName].Create), 0644)
				if err != nil {
					log.Printf("WARN: %s", err)
				}
				filename = fmt.Sprintf("%s/%d_add_%s_table.down.sql", c.OutputDir, timestamp, tableName)
				err = afero.WriteFile(c.FileSystem, filename, []byte(schemas[tableName].Drop), 0644)
				if err != nil {
					log.Printf("WARN: %s", err)
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

	return nil
}
