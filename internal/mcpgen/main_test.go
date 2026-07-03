// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// parseClientFuncs parses a synthetic api/client file and returns its func decls.
func parseClientFuncs(t *testing.T, src string) (*token.FileSet, []*ast.FuncDecl) {
	t.Helper()
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "client.go", "package client\n"+src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic client source: %v", err)
	}
	var fns []*ast.FuncDecl
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			fns = append(fns, fn)
		}
	}
	return fset, fns
}

// TestCallSiteMatchesBothWireCallShapes guards the G2 refactor
// (Oblikovati/Oblikovati#1650): the generator must extract the same
// method constant + request DTO from the legacy untyped body and from
// the generic call[Resp] body, and ignore unrelated calls.
func TestCallSiteMatchesBothWireCallShapes(t *testing.T) {
	src := `
func (w W) Untyped(args wire.CreateArgs) (wire.CreateResult, error) {
	var r wire.CreateResult
	return r, w.c.call(wire.MethodCreate, args, &r)
}
func (w W) Generic(args wire.CreateArgs) (wire.CreateResult, error) {
	return call[wire.CreateResult](w.c, wire.MethodCreate, args)
}
func (w W) GenericNilReq() (wire.ListResult, error) {
	return call[wire.ListResult](w.c, wire.MethodList, nil)
}
func (w W) GenericLiteralReq(name string) (wire.InfoResult, error) {
	return call[wire.InfoResult](w.c, wire.MethodGet, wire.NameArgs{Name: name})
}
func (w W) NotAWireCall() error {
	return other[int](w.c, wire.MethodX, nil)
}
`
	want := map[string]struct{ method, input string }{
		"Untyped":           {"MethodCreate", "wire.CreateArgs"},
		"Generic":           {"MethodCreate", "wire.CreateArgs"},
		"GenericNilReq":     {"MethodList", ""},
		"GenericLiteralReq": {"MethodGet", "wire.NameArgs"},
		"NotAWireCall":      {"", ""},
	}
	fset, fns := parseClientFuncs(t, src)
	for _, fn := range fns {
		method, input := callSite(fset, fn)
		w := want[fn.Name.Name]
		if method != w.method || input != w.input {
			t.Errorf("callSite(%s) = (%q, %q), want (%q, %q)",
				fn.Name.Name, method, input, w.method, w.input)
		}
	}
}
