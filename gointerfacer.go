package gointerfacer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"code.google.com/p/go.tools/imports"
)

// FindInterface returns the import path and identifier of an interface.
// For example, given "http.ResponseWriter", FindInterface returns
// "net/http", "ResponseWriter".
func FindInterface(iface string) (path string, pkg string, id string, err error) {
	if len(strings.Fields(iface)) != 1 {
		return "", "", "", fmt.Errorf("couldn't parse interface: %s", iface)
	}

	// Let goimports do the heavy lifting.
	src := []byte("package hack\n" + "var i " + iface)
	imp, err := imports.Process("", src, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("couldn't parse interface: %s", iface)
	}

	// imp should now contain an appropriate import.
	// Parse out the import and the identifier.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", imp, 0)
	if err != nil {
		panic(err)
	}
	if len(f.Imports) == 0 {
		return "", "", "", fmt.Errorf("unrecognized interface: %s", iface)
	}
	raw := f.Imports[0].Path.Value   // "io"
	path, err = strconv.Unquote(raw) // io
	if err != nil {
		panic(err)
	}
	decl := f.Decls[1].(*ast.GenDecl)      // var i io.Reader
	spec := decl.Specs[0].(*ast.ValueSpec) // i io.Reader
	sel := spec.Type.(*ast.SelectorExpr)   // io.Reader
	pkg = sel.X.(*ast.Ident).Name          // io
	id = sel.Sel.Name                      // Reader
	return path, pkg, id, nil
}

// Pkg is a parsed build.Package.
type Pkg struct {
	*build.Package
	*token.FileSet
}

// TypeSpec locates the *ast.TypeSpec for type id in the import path.
func TypeSpec(path string, id string) (Pkg, *ast.TypeSpec, error) {
	pkg, err := build.Import(path, "", 0)
	if err != nil {
		return Pkg{}, nil, fmt.Errorf("couldn't find package %s: %v", path, err)
	}

	fset := token.NewFileSet() // share one fset across the whole package
	for _, file := range pkg.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(pkg.Dir, file), nil, 0)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			decl, ok := decl.(*ast.GenDecl)
			if !ok || decl.Tok != token.TYPE {
				continue
			}
			for _, spec := range decl.Specs {
				spec := spec.(*ast.TypeSpec)
				if spec.Name.Name != id {
					continue
				}
				return Pkg{Package: pkg, FileSet: fset}, spec, nil
			}
		}
	}
	return Pkg{}, nil, fmt.Errorf("type %s not found in %s", id, path)
}

// gofmt pretty-prints e.
func (p Pkg) gofmt(e ast.Expr) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, p.FileSet, e)
	return buf.String()
}

// FullType returns the fully qualified type of e.
// Examples, assuming package net/http:
//  FullType(int) => "int"
//  FullType(Handler) => "http.Handler"
//  FullType(io.Reader) => "io.Reader"
//  FullType(*Request) => "*http.Request"
func (p Pkg) FullType(e ast.Expr) string {
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			// Using TypeSpec instead of IsExported here would be
			// more accurate, but it'd be crazy expensive, and if
			// the type isn't exported, there's no point trying
			// to implement it anyway.
			if n.IsExported() {
				n.Name = p.Package.Name + "." + n.Name
			}
		case *ast.SelectorExpr:
			return false
		}
		return true
	})
	return p.gofmt(e)
}

func (p Pkg) Params(field *ast.Field) []Param {
	var params []Param
	typ := p.FullType(field.Type)
	for _, name := range field.Names {
		params = append(params, Param{Name: name.Name, Type: typ})
	}
	// Handle anonymous params
	if len(params) == 0 {
		params = []Param{Param{Type: typ}}
	}
	return params
}

// Method represents a method signature.
type Method struct {
	Recv string
	Func
}

// Func represents a function signature.
type Func struct {
	Name   string
	Params []Param
	Res    []Param
}

// Param represents a parameter in a function or method signature.
type Param struct {
	Name string
	Type string
}

func (p Pkg) FuncSig(f *ast.Field) Func {
	fn := Func{Name: f.Names[0].Name}
	typ := f.Type.(*ast.FuncType)
	if typ.Params != nil {
		for _, field := range typ.Params.List {
			fn.Params = append(fn.Params, p.Params(field)...)
		}
	}
	if typ.Results != nil {
		for _, field := range typ.Results.List {
			fn.Res = append(fn.Res, p.Params(field)...)
		}
	}
	return fn
}

// Functions returns the set of methods required to implement iface.
// It is called Functions rather than methods because the
// function descriptions are functions; there is no receiver.
func Functions(iface string) ([]Func, error) {
	// Locate the interface.
	path, _, id, err := FindInterface(iface)
	if err != nil {
		return nil, err
	}

	// Parse the package and find the interface declaration.
	p, spec, err := TypeSpec(path, id)
	if err != nil {
		return nil, fmt.Errorf("interface %s not found: %s", iface, err)
	}
	idecl, ok := spec.Type.(*ast.InterfaceType)
	if !ok {
		return nil, fmt.Errorf("not an interface: %s", iface)
	}

	if idecl.Methods == nil {
		return nil, fmt.Errorf("empty interface: %s", iface)
	}

	var fns []Func
	for _, fndecl := range idecl.Methods.List {
		if len(fndecl.Names) == 0 {
			// Embedded interface: recurse
			embedded, err := Functions(p.FullType(fndecl.Type))
			if err != nil {
				return nil, err
			}
			fns = append(fns, embedded...)
			continue
		}

		fn := p.FuncSig(fndecl)
		fns = append(fns, fn)
	}
	return fns, nil
}
