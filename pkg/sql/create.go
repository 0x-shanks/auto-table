package sql

import (
	"errors"
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/ast"
	d "github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/naoina/go-stringutil"
	goast "go/ast"
	"log"
	"strings"
)

const idCandidate = "id"

var intPrimitives = map[string]struct{}{"int8": {}, "int16": {}, "int32": {}, "int64": {}, "int": {}, "uint8": {}, "uint16": {}, "uint32": {}, "uint64": {}, "uint": {}}

type SQL struct {
	Table struct {
		Create string
		Drop   string
	}
	Record struct {
		FindAll string
		Find    string
		Create  string
		Delete  string
		Update  string
	}
}

func CreateSQL(dialect d.Dialect, isAutoID bool, marker string, filenames []string) (sqlMap map[string]*SQL, dependencyMap map[string]map[string]struct{}, err error) {
	tableASTMap, tableNames, dependencyMap, err := makeTableASTMap(dialect, isAutoID, marker, filenames)
	sqlMap = makeSQLMap(dialect, tableASTMap, tableNames)
	return
}

func makeTableASTMap(dialect d.Dialect, isAutoID bool, marker string, filenames []string) (tableASTMap map[string]*ast.Table, tableNames []string, dependencyMap map[string]map[string]struct{}, err error) {

	modelASTMap, err := parseFileToASTMap(filenames, marker)

	tableASTMap = map[string]*ast.Table{} // map [tableName]details

	// Map used to control the order in which tables are created during migration.
	// Keeps track of whether one table depends on another table. All tables are stored in the key of the first map. The value stores which tables depend on it.
	dependencyMap = map[string]map[string]struct{}{} // map[tableName]dependency

	for modelName, StructAST := range modelASTMap {

		var hasID bool    // Whether or not this struct has an ID field
		var idType string // Type of ID field
		dependencyMap[modelName] = map[string]struct{}{}

		for _, fld := range StructAST.StructType.Fields.List {
			field, newHasID, newIDType, tErr := makeField(dialect, modelASTMap, tableASTMap, dependencyMap, modelName, fld, isAutoID, hasID, idType)
			if tErr != nil {
				log.Print(tErr)
				continue
			}
			hasID = newHasID
			idType = newIDType

			if tableASTMap[modelName] == nil {
				tableASTMap[modelName] = &ast.Table{
					Option: StructAST.Annotation.Option,
				}
			}
			tableASTMap[modelName].Fields = append(tableASTMap[modelName].Fields, field)
		}

	}

	// Get table names
	tableNames = make([]string, 0, len(tableASTMap))
	for name := range tableASTMap {
		tableNames = append(tableNames, name)
	}

	return
}

func parseFileToASTMap(filenames []string, marker string) (modelASTMap map[string]*ast.StructAST, err error) {
	modelASTMap = make(map[string]*ast.StructAST)

	for _, filename := range filenames {
		m, tErr := ast.MakeStructASTMap(filename, marker)
		if tErr != nil {
			err = tErr
			return
		}
		for k, v := range m {
			modelASTMap[k] = v
		}
	}

	return
}

func makeField(
	dialect d.Dialect,
	modelASTMap map[string]*ast.StructAST,
	tableASTMap map[string]*ast.Table, // With side effects
	dependencyMap map[string]map[string]struct{}, // With side effects
	modelName string,
	fld *goast.Field,
	isAutoID bool,
	hasID bool,
	idType string,
) (field *ast.Field, newHasID bool, newIDType string, err error) {
	typeStr, typeName, _, isArray, err := ast.DetectTypeName(fld)
	isPrimaryKey, isAutoIncrement, tHasID, tIDType, tErr := autoMakePrimaryFromID(isAutoID, fld, typeStr)
	if tErr != nil {
		err = tErr
		return
	}

	if hasID || tHasID {
		newHasID = true
		if tHasID {
			if idType != "" {
				err = errors.New("multiple IDs exist")
				return
			}
			newHasID = tHasID
			newIDType = tIDType
		} else {
			newHasID = hasID
			newIDType = idType
		}
	}

	fieldName, foreignKey, tErr := autoMakeForeignRelation(dialect, modelASTMap, tableASTMap, dependencyMap, modelName, fld, typeName, isArray, newHasID, newIDType)
	if tErr != nil {
		err = tErr
		return
	}

	field, err = ast.NewField(dialect, modelName, typeStr, fieldName, fld, foreignKey, isPrimaryKey, isAutoIncrement)
	if err != nil {
		return
	}
	if field.Ignore {
		err = fmt.Errorf("this field will be ignored: %s.%s", modelName, field.Name)
		return
	}

	if !(goast.IsExported(field.Name) || (field.Name == "_" && field.Name != field.Column)) {
		err = fmt.Errorf("this field has not been exported: %s.%s", modelName, field.Name)
		return
	}

	return
}

func autoMakePrimaryFromID(isAutoID bool, fld *goast.Field, typeStr string) (isPrimaryKey bool, isAutoIncrement bool, hasID bool, idType string, err error) {
	// Check if there is a field named id
	if strings.ToLower(fld.Names[0].Name) == idCandidate {
		if isAutoID {
			isPrimaryKey = true
			_, isAutoIncrement = intPrimitives[typeStr]
			hasID = true
			idType = typeStr
		}
	}
	return
}

func autoMakeForeignRelation(
	dialect d.Dialect,
	modelASTMap map[string]*ast.StructAST,
	tableASTMap map[string]*ast.Table, // With side effects
	dependencyMap map[string]map[string]struct{}, // With side effects
	modelName string,
	fld *goast.Field,
	typeName string,
	isArray bool,
	hasID bool,
	idType string,
) (fieldName *string, foreignKey *ast.ForeignKey, err error) {
	// Check to see if it is a dependent model
	if parent, ok := modelASTMap[strings.ToLower(typeName)]; ok {
		if isArray {
			// many-to-many

			// Make cross reference table
			crossReference := fmt.Sprintf("%s_%s", modelName, strings.ToLower(typeName))
			tableASTMap[crossReference] = &ast.Table{}
			dependencyMap[crossReference] = map[string]struct{}{}

			// self field
			sFieldName := fmt.Sprintf("%s_id", modelName)
			if !hasID {
				err = fmt.Errorf("%s doesn't have %s", modelName, idCandidate)
				return
			}
			sForeignKey := &ast.ForeignKey{
				Table:  modelName,
				Column: idCandidate,
			}
			dependencyMap[crossReference][modelName] = struct{}{}
			f, fErr := ast.NewField(dialect, modelName, idType, &sFieldName, fld, sForeignKey, true, false)
			if fErr != nil {
				err = fErr
				return
			}
			tableASTMap[crossReference].Fields = append(tableASTMap[crossReference].Fields, f)

			// parent field
			var pTypeStr string
			var pFieldName *string
			var pForeignKey *ast.ForeignKey
			for _, f := range parent.StructType.Fields.List {
				// Set foreign key
				if strings.ToLower(f.Names[0].Name) == idCandidate {

					pTypeStr, _, _, _, err = ast.DetectTypeName(f.Type)
					f := fmt.Sprintf("%v%v", stringutil.ToUpperCamelCase(fld.Names[0].Name), stringutil.ToUpperCamelCase(idCandidate))
					pFieldName = &f
					pForeignKey = &ast.ForeignKey{
						Table:  parent.Name,
						Column: idCandidate,
					}
					dependencyMap[crossReference][stringutil.ToSnakeCase(parent.Name)] = struct{}{}
				}
			}
			f, fErr = ast.NewField(dialect, modelName, pTypeStr, pFieldName, fld, pForeignKey, true, false)
			if fErr != nil {
				err = fErr
				return
			}
			tableASTMap[crossReference].Fields = append(tableASTMap[crossReference].Fields, f)

			// This is a case of an cross reference table, so no columns are added.
			return
		} else {
			// one-to-many
			for _, f := range parent.StructType.Fields.List {
				// Set foreign key
				if strings.ToLower(f.Names[0].Name) == idCandidate {
					f := fmt.Sprintf("%v%v", stringutil.ToUpperCamelCase(fld.Names[0].Name), stringutil.ToUpperCamelCase(idCandidate))
					fieldName = &f
					foreignKey = &ast.ForeignKey{
						Table:  parent.Name,
						Column: idCandidate,
					}
					dependencyMap[modelName][stringutil.ToSnakeCase(parent.Name)] = struct{}{}
				}
			}
		}
	}
	return
}

func makeSQLMap(dialect d.Dialect, tableASTMap map[string]*ast.Table, tableNames []string) (sqlMap map[string]*SQL) {
	sqlMap = map[string]*SQL{} // map[tableName]schema
	for _, name := range tableNames {
		tbl := tableASTMap[name]
		fields := make([]d.Field, len(tbl.Fields))
		for i, f := range tbl.Fields {
			fields[i] = f.ToField()
		}
		pks := ast.MakePrimaryKeyColumns(tbl.Fields)
		pkColumns := make([]string, len(pks))
		for i, pk := range pks {
			pkColumns[i] = pk.ToField().Name
		}
		fksColumns := ast.MakeForeignKeyColumns(tbl.Fields)
		t := d.Table{
			Name:        name,
			Fields:      fields,
			PrimaryKeys: pkColumns,
			ForeignKeys: fksColumns,
			Option:      tbl.Option,
		}
		createTableSQL := strings.Join(dialect.CreateTableSQL(t), "")
		dropTableSQL := strings.Join(dialect.DropTableSQL(t), "")
		findAllSQL := strings.Join(dialect.FindAllSQL(t), "")
		findSQL := strings.Join(dialect.FindSQL(t), "")
		createSQL := strings.Join(dialect.CreateSQL(t), "")
		deleteSQL := strings.Join(dialect.DeleteSQL(t), "")
		updateSQL := strings.Join(dialect.UpdateSQL(t), "")

		sqlMap[name] = &SQL{
			Table: struct {
				Create string
				Drop   string
			}{
				Create: createTableSQL,
				Drop:   dropTableSQL,
			},
			Record: struct {
				FindAll string
				Find    string
				Create  string
				Delete  string
				Update  string
			}{
				FindAll: findAllSQL,
				Find:    findSQL,
				Create:  createSQL,
				Delete:  deleteSQL,
				Update:  updateSQL,
			},
		}
	}
	return
}
