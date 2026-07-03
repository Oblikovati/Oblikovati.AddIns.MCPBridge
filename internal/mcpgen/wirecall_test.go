// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// funcDeclFromSource parses a single Go function declaration from src (a full file
// body) and returns it — the fixture for exercising the AST matchers on a real decl.
func funcDeclFromSource(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatalf("no func decl in fixture:\n%s", src)
	return nil
}

// TestCallSiteMatchesLegacyForm pins the pre-G2 delegate shape recv.call(wire.M, arg, &r):
// the generator must still read its method constant and request DTO from the body.
func TestCallSiteMatchesLegacyForm(t *testing.T) {
	fn := funcDeclFromSource(t, `package p
func (w WorkPoints) Create(args wire.CreateWorkPointArgs) (wire.CreateWorkPointResult, error) {
	var r wire.CreateWorkPointResult
	return r, w.c.call(wire.MethodWorkPointsCreate, args, &r)
}`)
	method, input := callSite(token.NewFileSet(), fn)
	if method != "MethodWorkPointsCreate" {
		t.Errorf("method = %q, want MethodWorkPointsCreate", method)
	}
	if input != "wire.CreateWorkPointArgs" {
		t.Errorf("input = %q, want wire.CreateWorkPointArgs", input)
	}
}

// TestCallSiteMatchesGenericForm is the G2 (#1650) regression: after the body collapses to
// call[Resp](recv, wire.M, arg), the leading receiver shifts the method/request one slot
// right, and the generator must read them from the new positions — else every converted tool
// silently vanishes from tools_generated.go.
func TestCallSiteMatchesGenericForm(t *testing.T) {
	fn := funcDeclFromSource(t, `package p
func (w WorkPoints) Create(args wire.CreateWorkPointArgs) (wire.CreateWorkPointResult, error) {
	return call[wire.CreateWorkPointResult](w.c, wire.MethodWorkPointsCreate, args)
}`)
	method, input := callSite(token.NewFileSet(), fn)
	if method != "MethodWorkPointsCreate" {
		t.Errorf("method = %q, want MethodWorkPointsCreate", method)
	}
	if input != "wire.CreateWorkPointArgs" {
		t.Errorf("input = %q, want wire.CreateWorkPointArgs", input)
	}
}

// TestCallSiteIgnoresUnrelatedCalls guards against false positives: a body whose only call is
// neither delegate shape yields no method, so a non-wire helper is never registered as a tool.
func TestCallSiteIgnoresUnrelatedCalls(t *testing.T) {
	fn := funcDeclFromSource(t, `package p
func (w WorkPoints) Touch() error {
	return w.c.flush(wire.MethodWorkPointsCreate)
}`)
	if method, _ := callSite(token.NewFileSet(), fn); method != "" {
		t.Errorf("method = %q, want empty for a non-delegate call", method)
	}
}
