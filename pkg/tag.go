package pkg

import (
	"bufio"
	"fmt"
	"github.com/hourglasshoro/auto-table/pkg/dialect"
	"reflect"
	"strings"
)

const (
	tagDefault       = "default"
	tagPrimaryKey    = "pk"
	tagAutoIncrement = "autoincrement"
	tagIndex         = "index"
	tagUnique        = "unique"
	tagColumn        = "column"
	tagType          = "type"
	tagNull          = "null"
	tagExtra         = "extra"
	tagIgnore        = "-"
)

func parseStructTag(d dialect.Dialect, f *field, tag reflect.StructTag) error {
	autoTable := tag.Get("autoTable")
	if autoTable == "" {
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(autoTable))
	scanner.Split(tagOptionSplit)
	for scanner.Scan() {
		opt := scanner.Text()
		optval := strings.SplitN(opt, ":", 2)
		switch optval[0] {
		case tagDefault:
			if len(optval) > 1 {
				f.Default = optval[1]
			}
		case tagPrimaryKey:
			f.PrimaryKey = true
		case tagAutoIncrement:
			f.AutoIncrement = true
		case tagIndex:
			if len(optval) == 2 {
				f.RawIndexes = append(f.RawIndexes, optval[1])
			} else {
				f.RawIndexes = append(f.RawIndexes, "")
			}
		case tagUnique:
			if len(optval) == 2 {
				f.RawUniques = append(f.RawUniques, optval[1])
			} else {
				f.RawUniques = append(f.RawUniques, "")
			}
		case tagIgnore:
			f.Ignore = true
		case tagColumn:
			if len(optval) < 2 {
				return fmt.Errorf("`column` tag must specify the parameter")
			}
			f.Column = optval[1]
		case tagType:
			if len(optval) < 2 {
				return fmt.Errorf("`type` tag must specify the parameter")
			}
			f.Type = optval[1]
		case tagNull:
			f.Nullable = true
		case tagExtra:
			if len(optval) < 2 {
				return fmt.Errorf("`extra` tag must specify the parameter")
			}
			f.Extra = optval[1]
		default:
			return fmt.Errorf("unknown option: `%s'", opt)
		}
	}
	return scanner.Err()
}

func tagOptionSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	var inParenthesis bool
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ',':
			if !inParenthesis {
				return i + 1, data[:i], nil
			}
		case '(':
			inParenthesis = true
		case ')':
			inParenthesis = false
		}
	}
	return 0, data, bufio.ErrFinalToken
}
