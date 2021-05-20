package ast

import (
	"fmt"
	"github.com/naoina/go-stringutil"
	"go/ast"
	"go/parser"
	"go/token"
)

type StructAST struct {
	Name       string
	StructType *ast.StructType
	Annotation *annotation
}

func MakeStructASTMap(filename string, marker string) (map[string]*StructAST, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	structASTMap := map[string]*StructAST{}
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)

		if !ok || d.Tok != token.TYPE || d.Doc == nil {
			continue
		}
		// General Declaration && isType true && //+marker

		annotation, err := parseAnnotation(d.Doc, marker)
		if err != nil {
			return nil, err
		}
		if annotation == nil {
			continue
		}
		for _, spec := range d.Specs {
			s, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			t, ok := s.Type.(*ast.StructType)
			if !ok {
				continue
			}
			st := &StructAST{
				Name:       s.Name.Name,
				StructType: t,
				Annotation: annotation,
			}
			if annotation.Table != "" {
				structASTMap[annotation.Table] = st
			} else {
				structASTMap[stringutil.ToSnakeCase(s.Name.Name)] = st
			}
		}
	}
	return structASTMap, nil
}

func DetectTypeName(n ast.Node) (str string, name string, isPtr bool, isArray bool, err error) {
	switch t := n.(type) {
	case *ast.Field:
		return DetectTypeName(t.Type)
	case *ast.Ident:
		str = t.Name
		name = t.Name
		return
	case *ast.SelectorExpr:
		xStr, _, _, _, xErr := DetectTypeName(t.X)
		if xErr != nil {
			err = xErr
			return
		}
		str = xStr + "." + t.Sel.Name
		name = xStr + "." + t.Sel.Name
		return
	case *ast.StarExpr:
		xStr, _, _, _, xErr := DetectTypeName(t.X)
		if xErr != nil {
			err = xErr
			return
		}
		str = "*" + xStr
		name = xStr
		isPtr = true
		return
	case *ast.ArrayType:
		eltStr, _, _, _, eltErr := DetectTypeName(t.Elt)
		if eltErr != nil {
			err = eltErr
			return
		}
		str = "[]" + eltStr
		name = eltStr
		isArray = true
		return
	default:
		err = fmt.Errorf("auto-table: BUG: unknown type %T", t)
		return
	}
}
