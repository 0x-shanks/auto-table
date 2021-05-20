package ast

import (
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"github.com/naoina/go-stringutil"
	"go/ast"
	"reflect"
	"strconv"
	"strings"
)

type Field struct {
	Table         string
	Name          string
	GoType        string
	Type          string
	Column        string
	Comment       string
	RawIndexes    []string
	RawUniques    []string
	PrimaryKey    bool
	AutoIncrement bool
	Ignore        bool
	Default       string
	Extra         string
	Nullable      bool
	ForeignKey    *ForeignKey
}

type ForeignKey struct {
	Table  string
	Column string
}

func NewField(
	d dialect.Dialect,
	tableName string,
	typeName string,
	fieldName *string,
	f *ast.Field,
	foreignKey *ForeignKey,
	primaryKey bool,
	autoIncrement bool,
) (*Field, error) {
	ret := &Field{
		Table:  tableName,
		GoType: typeName,
	}
	if len(f.Names) > 0 && f.Names[0] != nil {
		ret.Name = f.Names[0].Name
		if fieldName != nil {
			ret.Name = *fieldName
		}
	}
	ret.PrimaryKey = primaryKey
	ret.AutoIncrement = autoIncrement
	if ret.IsEmbedded() {
		return ret, nil
	}
	if f.Tag != nil {
		s, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			return nil, err
		}
		if err := parseStructTag(ret, reflect.StructTag(s)); err != nil {
			return nil, err
		}
	}
	if f.Comment != nil {
		ret.Comment = strings.TrimSpace(f.Comment.Text())
	}
	if ret.Column == "" {
		ret.Column = stringutil.ToSnakeCase(ret.Name)
	}
	if !ret.Nullable {
		if ret.GoType[0] == '*' {
			ret.Nullable = true
		} else {
			ret.Nullable = d.IsNullable(strings.TrimLeft(ret.GoType, "*"))
		}
	}
	if foreignKey != nil {
		ret.ForeignKey = foreignKey
	}
	var colType string
	if ret.Type == "" {
		colType = strings.TrimLeft(ret.GoType, "*")
	} else {
		colType = ret.Type
	}
	ret.Type = d.ColumnType(colType)
	return ret, nil
}

func (f *Field) IsEmbedded() bool {
	return f.Name == ""
}

func (f *Field) ToField() dialect.Field {
	return dialect.Field{
		Table:         f.Table,
		Name:          f.Column,
		Type:          f.Type,
		Comment:       f.Comment,
		AutoIncrement: f.AutoIncrement,
		Default:       f.Default,
		Extra:         f.Extra,
		Nullable:      f.Nullable,
	}
}

func MakePrimaryKeyColumns(fields []*Field) (pks []*Field) {
	for _, f := range fields {
		if f.PrimaryKey {
			pks = append(pks, f)
		}
	}
	return
}

func MakeForeignKeyColumns(fields []*Field) (fks map[string]dialect.ForeignKey) {
	fks = map[string]dialect.ForeignKey{} //map[fieldName]reference
	for _, f := range fields {
		if f.ForeignKey != nil {
			fks[f.Name] = dialect.ForeignKey{
				Table:  f.ForeignKey.Table,
				Column: f.ForeignKey.Column,
			}
		}
	}

	return
}
