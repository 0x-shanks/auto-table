package dialect

import (
	"database/sql"
	"fmt"
	"github.com/naoina/go-stringutil"
	"strings"
)

var _ PrimaryKeyModifier = &MySQL{}

var (
	mysqlColumnTypes = []*ColumnType{
		{
			Types:           []string{"VARCHAR", "TEXT", "MEDIUMTEXT", "LONGTEXT", "CHAR"},
			GoTypes:         []string{"string"},
			GoNullableTypes: []string{"*string", "sql.NullString"},
		},
		{
			Types:           []string{"VARBINARY", "BINARY"},
			GoTypes:         []string{"[]byte"},
			GoNullableTypes: []string{"[]byte"},
		},
		{
			Types:           []string{"INT", "MEDIUMINT"},
			GoTypes:         []string{"int", "int32"},
			GoUnsignedTypes: []string{"uint", "uint32"},
		},
		{
			Types:           []string{"TINYINT"},
			GoTypes:         []string{"int8"},
			GoUnsignedTypes: []string{"uint8"},
		},
		{
			Types:           []string{"TINYINT(1)"},
			GoTypes:         []string{"bool"},
			GoNullableTypes: []string{"*bool", "sql.NullBool"},
		},
		{
			Types:           []string{"SMALLINT"},
			GoTypes:         []string{"int16"},
			GoUnsignedTypes: []string{"uint16"},
		},
		{
			Types:           []string{"BIGINT"},
			GoTypes:         []string{"int64"},
			GoUnsignedTypes: []string{"uint64"},
			GoNullableTypes: []string{"*int64", "sql.NullInt64"},
		},
		{
			Types:           []string{"DOUBLE", "FLOAT", "DECIMAL"},
			GoTypes:         []string{"float64", "float32"},
			GoNullableTypes: []string{"*float64", "sql.NullFloat64"},
		},
		{
			Types:           []string{"DATETIME"},
			GoTypes:         []string{"time.Time"},
			GoNullableTypes: []string{"*time.Time", "mysql.NullTime", "gorp.NullTime"},
		},
	}
)

type MySQL struct {
	columnTypeMap   map[string]*ColumnType
	nullableTypeMap map[string]struct{}
}

func NewMySQL() Dialect {
	d := &MySQL{
		columnTypeMap:   map[string]*ColumnType{},
		nullableTypeMap: map[string]struct{}{},
	}

	for _, types := range [][]*ColumnType{mysqlColumnTypes} {
		for _, t := range types {
			for _, tt := range t.allGoTypes() {
				d.columnTypeMap[tt] = t
			}
			for _, tt := range t.filteredNullableGoTypes() {
				d.nullableTypeMap[tt] = struct{}{}
			}
		}
	}
	return d
}

func (d *MySQL) ColumnType(name string) string {
	var unsigned bool
	if t, ok := d.columnTypeMap[name]; ok {
		name, _, unsigned, _ = t.findType(name)
	}
	name = d.defaultColumnType(name)
	if unsigned {
		name += " UNSIGNED"
	}
	return strings.ToUpper(name)
}

func (d *MySQL) GoType(name string, nullable bool) string {
	name = strings.ToUpper(name)
	var unsigned bool
	if i := strings.IndexByte(name, ' '); i >= 0 {
		name, unsigned = name[:i], name[i+1:] == "UNSIGNED"
	}
	for _, t := range mysqlColumnTypes {
		if typ, found := t.findGoType(name, nullable, unsigned); found {
			return typ
		}
	}
	if strings.IndexByte(name, '(') >= 0 {
		return d.GoType(trimParens(name), nullable)
	}
	return "interface{}"
}

func (d *MySQL) IsNullable(name string) bool {
	_, ok := d.nullableTypeMap[name]
	return ok
}

func (d *MySQL) ImportPackage(schema ColumnSchema) string {
	switch schema.DataType() {
	case "datetime":
		return "time"
	}
	return ""
}

func (d *MySQL) Quote(s string) string {
	return fmt.Sprintf("`%s`", strings.Replace(s, "`", "``", -1))
}

func (d *MySQL) QuoteString(s string) string {
	return fmt.Sprintf("'%s'", strings.Replace(s, "'", "''", -1))
}

func (d *MySQL) CreateTableSQL(table Table) []string {
	columns := make([]string, len(table.Fields))
	for i, f := range table.Fields {
		columns[i] = d.columnSQL(f)
	}
	//TODO: not use append
	columns = append(columns, "`created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
	columns = append(columns, "`updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP")

	if len(table.PrimaryKeys) > 0 {
		pkColumns := make([]string, len(table.PrimaryKeys))
		for i, pk := range table.PrimaryKeys {
			pkColumns[i] = d.Quote(pk)
		}
		columns = append(columns, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkColumns, ", ")))
	}
	if len(table.ForeignKeys) > 0 {
		for name, reference := range table.ForeignKeys {
			columns = append(columns,
				fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)",
					d.Quote(stringutil.ToSnakeCase(name)),
					d.Quote(stringutil.ToSnakeCase(reference.Table)),
					d.Quote(reference.Column)))
		}
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n"+
		"  %s\n"+
		");", d.Quote(table.Name), strings.Join(columns, ",\n  "))
	if table.Option != "" {
		query += " " + table.Option
	}
	return []string{query}
}

func (d *MySQL) DropTableSQL(table Table) []string {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s;", d.Quote(table.Name))
	if table.Option != "" {
		query += " " + table.Option
	}
	return []string{query}
}

func (d *MySQL) FindAllSQL(table Table) []string {
	columns := make([]string, len(table.Fields))
	for i, f := range table.Fields {
		columns[i] = d.Quote(f.Name)
	}
	columns = append(columns, "`created_at`")
	columns = append(columns, "`updated_at`")
	query := fmt.Sprintf("SELECT %s FROM %s;", strings.Join(columns, ", "), d.Quote(table.Name))
	return []string{query}
}

func (d *MySQL) FindSQL(table Table) []string {
	columns := make([]string, len(table.Fields))
	for i, f := range table.Fields {
		columns[i] = d.Quote(f.Name)
	}
	columns = append(columns, "`created_at`")
	columns = append(columns, "`updated_at`")
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?;", strings.Join(columns, ", "), d.Quote(table.Name), columns[0])
	return []string{query}
}

func (d *MySQL) CreateSQL(table Table) []string {
	columns := make([]string, len(table.Fields))
	values := make([]string, len(table.Fields))
	for i, f := range table.Fields {
		columns[i] = d.Quote(f.Name)
		values[i] = "?"
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", d.Quote(table.Name), strings.Join(columns, ", "), strings.Join(values, ", "))
	return []string{query}
}

func (d *MySQL) DeleteSQL(table Table) []string {
	columns := make([]string, len(table.Fields))
	for i, f := range table.Fields {
		columns[i] = d.Quote(f.Name)
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?;", d.Quote(table.Name), d.Quote(table.Fields[0].Name))
	return []string{query}
}

func (d *MySQL) UpdateSQL(table Table) []string {
	set := make([]string, len(table.Fields))
	for i, f := range table.Fields {
		if i == 0 {
			continue
		}
		set[i] = fmt.Sprintf("%s = ?", d.Quote(f.Name))
	}
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?;", d.Quote(table.Name), strings.Join(set, " "), d.Quote(table.Fields[0].Name))
	return []string{query}
}

func (d *MySQL) AddColumnSQL(field Field) []string {
	return []string{fmt.Sprintf("ALTER TABLE %s ADD %s", d.Quote(field.Table), d.columnSQL(field))}
}

func (d *MySQL) DropColumnSQL(field Field) []string {
	return []string{fmt.Sprintf("ALTER TABLE %s DROP %s", d.Quote(field.Table), d.Quote(field.Name))}
}

func (d *MySQL) ModifyColumnSQL(oldField, newField Field) []string {
	return []string{fmt.Sprintf("ALTER TABLE %s CHANGE %s %s", d.Quote(newField.Table), d.Quote(oldField.Name), d.columnSQL(newField))}
}

func (d *MySQL) ModifyPrimaryKeySQL(oldPrimaryKeys, newPrimaryKeys []Field) []string {
	var tableName string
	if len(newPrimaryKeys) > 0 {
		tableName = newPrimaryKeys[0].Table
	} else {
		tableName = oldPrimaryKeys[0].Table
	}
	var specs []string
	if len(oldPrimaryKeys) > 0 {
		specs = append(specs, "DROP PRIMARY KEY")
	}
	pkColumns := make([]string, len(newPrimaryKeys))
	for i, pk := range newPrimaryKeys {
		pkColumns[i] = d.Quote(pk.Name)
	}
	specs = append(specs, fmt.Sprintf("ADD PRIMARY KEY (%s)", strings.Join(pkColumns, ", ")))
	return []string{fmt.Sprintf("ALTER TABLE %s %s", d.Quote(tableName), strings.Join(specs, ", "))}
}

func (d *MySQL) CreateIndexSQL(index Index) []string {
	columns := make([]string, len(index.Columns))
	for i, c := range index.Columns {
		columns[i] = d.Quote(c)
	}
	indexName := d.Quote(index.Name)
	tableName := d.Quote(index.Table)
	column := strings.Join(columns, ",")
	if index.Unique {
		return []string{fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", indexName, tableName, column)}
	}
	return []string{fmt.Sprintf("CREATE INDEX %s ON %s (%s)", indexName, tableName, column)}
}

func (d *MySQL) DropIndexSQL(index Index) []string {
	return []string{fmt.Sprintf("DROP INDEX %s ON %s", d.Quote(index.Name), d.Quote(index.Table))}
}

func (d *MySQL) columnSQL(f Field) string {
	column := []string{d.Quote(f.Name), f.Type}
	if !f.Nullable {
		column = append(column, "NOT NULL")
	}
	if def := f.Default; def != "" {
		if d.isTextType(f) {
			def = d.QuoteString(def)
		}
		column = append(column, "DEFAULT", def)
	}
	if f.AutoIncrement {
		column = append(column, "AUTO_INCREMENT")
	}
	if f.Extra != "" {
		column = append(column, f.Extra)
	}
	if f.Comment != "" {
		column = append(column, "COMMENT", d.QuoteString(f.Comment))
	}
	return strings.Join(column, " ")
}

func (d *MySQL) isTextType(f Field) bool {
	typ := strings.ToUpper(f.Type)
	for _, t := range []string{"VARCHAR", "CHAR", "TEXT", "MIDIUMTEXT", "LONGTEXT"} {
		if strings.HasPrefix(typ, t) {
			return true
		}
	}
	return false
}

func (d *MySQL) defaultColumnType(name string) string {
	switch name := strings.ToUpper(name); name {
	case "BIT":
		return "BIT(1)"
	case "DECIMAL":
		return "DECIMAL(10,0)"
	case "VARCHAR":
		return "VARCHAR(255)"
	case "VARBINARY":
		return "VARBINARY(255)"
	case "CHAR":
		return "CHAR(1)"
	case "BINARY":
		return "BINARY(1)"
	case "YEAR":
		return "YEAR(4)"
	}
	return name
}

type mysqlIndexInfo struct {
	NonUnique int64
	IndexName string
}

type mysqlVersion struct {
	Major int
	Minor int
	Patch int
	Name  string
}

type mysqlTransaction struct {
	tx *sql.Tx
}

func (m *mysqlTransaction) Exec(sql string, args ...interface{}) error {
	_, err := m.tx.Exec(sql, args...)
	return err
}

func (m *mysqlTransaction) Commit() error {
	return m.tx.Commit()
}

func (m *mysqlTransaction) Rollback() error {
	return m.tx.Rollback()
}

func trimParens(s string) string {
	start, end := -1, -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '(' {
			start = i
			continue
		}
		if c == ')' {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return s
	}
	return s[:start] + s[end+1:]
}

var _ ColumnSchema = &mysqlColumnSchema{}

type mysqlColumnSchema struct {
	tableName              string
	columnName             string
	ordinalPosition        int64
	columnDefault          sql.NullString
	isNullable             string
	dataType               string
	characterMaximumLength *uint64
	characterOctetLength   sql.NullInt64
	numericPrecision       sql.NullInt64
	numericScale           sql.NullInt64
	datetimePrecision      sql.NullInt64
	columnType             string
	columnKey              string
	extra                  string
	columnComment          string
	nonUnique              int64
	indexName              string

	version *mysqlVersion
}

func (schema *mysqlColumnSchema) TableName() string {
	return schema.tableName
}

func (schema *mysqlColumnSchema) ColumnName() string {
	return schema.columnName
}

func (schema *mysqlColumnSchema) ColumnType() string {
	typ := schema.columnType
	switch schema.dataType {
	case "tinyint", "smallint", "mediumint", "int", "bigint":
		if typ == "tinyint(1)" {
			return typ
		}
		// NOTE: As of MySQL 8.0.17, the display width attribute is deprecated for integer data types.
		//		 See https://dev.mysql.com/doc/refman/8.0/en/numeric-type-syntax.html
		return trimParens(typ)
	}
	return typ
}

func (schema *mysqlColumnSchema) DataType() string {
	return schema.dataType
}

func (schema *mysqlColumnSchema) IsPrimaryKey() bool {
	return schema.columnKey == "PRI" && strings.ToUpper(schema.indexName) == "PRIMARY"
}

func (schema *mysqlColumnSchema) IsAutoIncrement() bool {
	return schema.extra == "auto_increment"
}

func (schema *mysqlColumnSchema) Index() (name string, unique bool, ok bool) {
	if schema.indexName != "" && !schema.IsPrimaryKey() {
		return schema.indexName, schema.nonUnique == 0, true
	}
	return "", false, false
}

func (schema *mysqlColumnSchema) Default() (string, bool) {
	if !schema.columnDefault.Valid {
		return "", false
	}
	def := schema.columnDefault.String
	v := schema.version
	// See https://mariadb.com/kb/en/library/information-schema-columns-table/
	if v.Name == "MariaDB" && v.Major >= 10 && v.Minor >= 2 && v.Patch >= 7 {
		// unquote string
		if len(def) > 0 && def[0] == '\'' {
			def = def[1:]
		}
		if len(def) > 0 && def[len(def)-1] == '\'' {
			def = def[:len(def)-1]
		}
		def = strings.Replace(def, "''", "'", -1) // unescape string
	}
	if def == "NULL" {
		return "", false
	}
	if schema.dataType == "datetime" && def == "0000-00-00 00:00:00" {
		return "", false
	}
	// Trim parenthesis from like "on update current_timestamp()".
	def = strings.TrimSuffix(def, "()")
	return def, true
}

func (schema *mysqlColumnSchema) IsNullable() bool {
	return strings.ToUpper(schema.isNullable) == "YES"
}

func (schema *mysqlColumnSchema) Extra() (string, bool) {
	if schema.extra == "" || schema.IsAutoIncrement() {
		return "", false
	}
	// Trim parenthesis from like "on update current_timestamp()".
	extra := strings.TrimSuffix(schema.extra, "()")
	extra = strings.ToUpper(extra)
	return extra, true
}

func (schema *mysqlColumnSchema) Comment() (string, bool) {
	return schema.columnComment, schema.columnComment != ""
}

func (schema *mysqlColumnSchema) isUnsigned() bool {
	return strings.Contains(schema.columnType, "unsigned")
}
