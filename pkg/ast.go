package pkg

import (
	"fmt"
	"github.com/naoina/go-stringutil"
	"go/ast"
	"go/parser"
	"go/token"
)

type structAST struct {
	Name       string
	StructType *ast.StructType
	Annotation *annotation
}

func makeStructASTMap(filename string) (map[string]*structAST, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	structASTMap := map[string]*structAST{}
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)

		if !ok || d.Tok != token.TYPE || d.Doc == nil {
			continue
		}
		// General Declaration && isType true && //+hoge

		annotation, err := parseAnnotation(d.Doc)
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
			st := &structAST{
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

func detectTypeName(n ast.Node) (str string, name string, isPtr bool, isArray bool, err error) {
	switch t := n.(type) {
	case *ast.Field:
		return detectTypeName(t.Type)
	case *ast.Ident:
		str = t.Name
		name = t.Name
		return
	case *ast.SelectorExpr:
		xStr, _, _, _, xErr := detectTypeName(t.X)
		if xErr != nil {
			err = xErr
			return
		}
		str = xStr + "." + t.Sel.Name
		name = xStr + "." + t.Sel.Name
		return
	case *ast.StarExpr:
		xStr, _, _, _, xErr := detectTypeName(t.X)
		if xErr != nil {
			err = xErr
			return
		}
		str = "*" + xStr
		name = xStr
		isPtr = true
		return
	case *ast.ArrayType:
		eltStr, _, _, _, eltErr := detectTypeName(t.Elt)
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
