package main

import (
	_ "embed"
	"fmt"
	"strings"
	"syscall/js"

	"go/ast"
	"go/parser"
	"go/token"

	"github.com/fatih/structtag"
)

var Indent = "    "

func getIdent(s string) string {
	switch s {
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64",
		"complex64", "complex128":
		return "number"
	}

	return s
}

func writeType(s *strings.Builder, t ast.Expr, depth int) {
	switch t := t.(type) {
	case *ast.ArrayType:
		writeType(s, t.Elt, depth)
		s.WriteString("[]")
	case *ast.StructType:
		s.WriteString("{\n")
		writeFields(s, t.Fields.List, depth+1)

		for i := 0; i < depth+1; i++ {
			s.WriteString(Indent)
		}
		s.WriteByte('}')
	case *ast.Ident:
		s.WriteString(getIdent(t.String()))
	case *ast.SelectorExpr:
		longType := fmt.Sprintf("%s.%s", t.X, t.Sel)
		switch longType {
		case "time.Time":
			s.WriteString("string")
		case "decimal.Decimal":
			s.WriteString("number")
		default:
			s.WriteString(longType)
		}
	default:
		panic(fmt.Errorf("unhandled: %s, %T", t, t))
	}
}

func writeFields(s *strings.Builder, fields []*ast.Field, depth int) {
	for _, f := range fields {
		for i := 0; i < depth+1; i++ {
			s.WriteString(Indent)
		}

		optional := false

		var name string
		if f.Tag != nil {
			tags, err := structtag.Parse(f.Tag.Value[1 : len(f.Tag.Value)-1])
			if err != nil {
				panic(err)
			}

			jsonTag, err := tags.Get("json")
			if err == nil {
				name = jsonTag.Name
				optional = jsonTag.HasOption("omitempty")
			}
		}

		if len(name) == 0 {
			if len(f.Names) != 0 && f.Names[0] != nil {
				name = f.Names[0].Name
			}
		}

		s.WriteString(name)

		switch t := f.Type.(type) {
		case *ast.StarExpr:
			optional = true
			f.Type = t.X
		}

		if optional {
			s.WriteByte('?')
		}

		s.WriteString(": ")

		writeType(s, f.Type, depth)

		s.WriteString(";\n")
	}
}

func Convert(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}

	var f ast.Node
	f, err := parser.ParseExprFrom(token.NewFileSet(), "editor.go", s, parser.SpuriousErrors)
	if err != nil {
		s = fmt.Sprintf(`package main

func main() {
	%s
}`, s)

		f, err = parser.ParseFile(token.NewFileSet(), "editor.go", s, parser.SpuriousErrors)
		if err != nil {
			panic(err)
		}
	}

	w := new(strings.Builder)
	name := "MyInterface"

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			name = x.Name
		case *ast.StructType:
			w.WriteString("declare interface ")
			w.WriteString(name)
			w.WriteString(" {\n")

			writeFields(w, x.Fields.List, 0)
			return false
		}
		return true
	})

	w.WriteByte('}')

	return w.String()
}

func main() {
	js.Global().Set("convert", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		defer func() {
			if r := recover(); r != nil {
				js.Global().Set("err", fmt.Sprintf("%s", r))
			}
		}()

		return js.ValueOf(Convert(args[0].String()))
	}))

	select {}
}
