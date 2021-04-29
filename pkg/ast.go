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

func detectTypeName(n ast.Node) (string, error) {
	switch t := n.(type) {
	case *ast.Field:
		return detectTypeName(t.Type)
	case *ast.Ident:
		return t.Name, nil
	case *ast.SelectorExpr:
		name, err := detectTypeName(t.X)
		if err != nil {
			return "", err
		}
		return name + "." + t.Sel.Name, nil
	case *ast.StarExpr:
		name, err := detectTypeName(t.X)
		if err != nil {
			return "", err
		}
		return "*" + name, nil
	case *ast.ArrayType:
		name, err := detectTypeName(t.Elt)
		if err != nil {
			return "", err
		}
		return "[]" + name, nil
	default:
		return "", fmt.Errorf("auto-table: BUG: unknown type %T", t)
	}
}
