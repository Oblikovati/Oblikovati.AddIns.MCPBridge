// SPDX-License-Identifier: GPL-2.0-only

// Command mcpgen generates bridge/tools_generated.go from the api/client mcp:
// annotations — the API is the single source of truth for the MCP tool surface.
//
// For every client method `func (...) M(...) (Result, error)` that ends in
// `c.call(wire.MethodX, ARG, &r)`, mcpgen reads the doc-comment directives:
//
//	mcp:tool <name>        expose as an MCP tool named <name>
//	mcp:summary <text>     the LLM-facing description
//	mcp:digest <fn>        forward + post-process the reply with bridge fn <fn>
//	mcp:custom <reason>    exposed but hand-written in the bridge (skipped here)
//	mcp:skip <reason>      intentionally not a tool (skipped here)
//
// The request DTO and result type come from the call site / signature, so the
// emitted registration is fully typed. Run via `go generate ./...`.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type tool struct {
	method  string // wire.Method constant name
	name    string // MCP tool name
	summary string
	digest  string // summarizer fn (digest tools only)
	input   string // request DTO, e.g. "wire.SaveDocumentArgs" or "" for noArgs
	output  string // result type, e.g. "wire.ListSketchesResult" (digest tools only)
	image   bool   // returns an image (capture tools): emit addCapture
}

func main() {
	apiDir := apiModuleDir()
	tools := collect(filepath.Join(apiDir, "client"))
	enforceParity(filepath.Join(apiDir, "wire"), tools)
	src := render(tools)
	out := "tools_generated.go"
	if err := os.WriteFile(out, src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("mcpgen: wrote %d tools to %s\n", len(tools), out)
}

// enforceParity fails generation if any wire method constant has no annotated client
// method behind it — the guarantee that the MCP surface stays at full API parity as
// the API grows. A method that should not be a tool must be annotated mcp:skip in
// api/client (which collect records); there is intentionally no silent gap.
func enforceParity(wireDir string, tools []tool) {
	covered := map[string]bool{"": true}
	for _, t := range tools {
		covered[t.method] = true
	}
	for _, m := range append(skipped, "") {
		covered[m] = true
	}
	var missing []string
	for _, m := range wireMethodConsts(wireDir) {
		if !covered[m] {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(os.Stderr, "mcpgen: %d wire method(s) have no MCP tool — annotate the client method with mcp:tool or mcp:skip:\n", len(missing))
		for _, m := range missing {
			fmt.Fprintln(os.Stderr, "  wire."+m)
		}
		os.Exit(1)
	}
}

// wireMethodConsts returns every `Method* = "..."` constant declared in api/wire.
func wireMethodConsts(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	fset := token.NewFileSet()
	var names []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(node, func(n ast.Node) bool {
			vs, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for _, nm := range vs.Names {
				if strings.HasPrefix(nm.Name, "Method") {
					names = append(names, nm.Name)
				}
			}
			return true
		})
	}
	return names
}

// apiModuleDir resolves the local oblikovati.org/api module directory (go.work in
// dev, the CI -replace otherwise), the same way the Makefile vendors the C header.
func apiModuleDir() string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "oblikovati.org/api")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "go list api module:", err)
		os.Exit(1)
	}
	return strings.TrimSpace(buf.String())
}

// skipped collects wire methods explicitly annotated mcp:skip (intentionally not a
// tool); enforceParity treats them as covered.
var skipped []string

func collect(dir string) []tool {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	fset := token.NewFileSet()
	seen := map[string]bool{}
	var tools []tool
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Fprintln(os.Stderr, "parse", path, err)
			os.Exit(1)
		}
		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Doc == nil {
				continue
			}
			directives := parseDirectives(fn.Doc.Text())
			method, input := callSite(fset, fn)
			if method == "" {
				continue
			}
			if _, skip := directives["skip"]; skip {
				skipped = append(skipped, method)
				continue
			}
			if directives["tool"] == "" || seen[method] {
				continue // unannotated, or dedup: one tool per wire method
			}
			seen[method] = true
			if v, ok := directives["input"]; ok && v != "" {
				input = v // override: a bridge-local input type for a cleaner schema
			}
			_, image := directives["image"]
			t := tool{
				method:  method,
				name:    directives["tool"],
				summary: directives["summary"],
				digest:  directives["digest"],
				input:   input,
				image:   image,
			}
			if t.digest != "" {
				t.output = resultType(fset, fn)
			}
			tools = append(tools, t)
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].name < tools[j].name })
	return tools
}

// parseDirectives pulls "mcp:<key> <value>" lines out of a doc comment.
func parseDirectives(doc string) map[string]string {
	d := map[string]string{}
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "mcp:")
		if !ok {
			continue
		}
		key, val, _ := strings.Cut(rest, " ")
		d[key] = strings.TrimSpace(val)
	}
	return d
}

// callSite returns the wire method constant and request DTO for the func's
// c.call(wire.MethodX, ARG, &r). DTO is "" for a nil / noArgs request.
func callSite(fset *token.FileSet, fn *ast.FuncDecl) (method, input string) {
	vars := map[string]ast.Expr{}
	if fn.Type.Params != nil {
		for _, p := range fn.Type.Params.List {
			for _, nm := range p.Names {
				vars[nm.Name] = p.Type
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range as.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && i < len(as.Rhs) {
					if cl, ok := as.Rhs[i].(*ast.CompositeLit); ok && cl.Type != nil {
						vars[id.Name] = cl.Type
					}
				}
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "call" || len(call.Args) < 2 {
			return true
		}
		ms, ok := call.Args[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := ms.X.(*ast.Ident); !ok || pkg.Name != "wire" {
			return true
		}
		method = ms.Sel.Name
		switch a := call.Args[1].(type) {
		case *ast.Ident:
			if a.Name != "nil" {
				if t, ok := vars[a.Name]; ok {
					input = exprString(fset, t)
				}
			}
		case *ast.CompositeLit:
			if a.Type != nil {
				input = exprString(fset, a.Type)
			}
		}
		return true
	})
	return method, input
}

// resultType returns the func's first (non-error) return type, the digest Out.
func resultType(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return ""
	}
	return exprString(fset, fn.Type.Results.List[0].Type)
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var sb strings.Builder
	_ = printer.Fprint(&sb, fset, e)
	return sb.String()
}

func render(tools []tool) []byte {
	var b strings.Builder
	b.WriteString("// SPDX-License-Identifier: GPL-2.0-only\n\n")
	b.WriteString("// Code generated by internal/mcpgen from the api/client mcp: annotations; DO NOT EDIT.\n")
	b.WriteString("// Regenerate with `go generate ./...`.\n\n")
	b.WriteString("package bridge\n\n")
	b.WriteString("import \"oblikovati.org/api/wire\"\n\n")
	b.WriteString("// registerGeneratedTools wires every annotated api/client method to an MCP tool.\n")
	b.WriteString("func (s *Server) registerGeneratedTools() {\n")
	for _, t := range tools {
		in := t.input
		if in == "" {
			in = "noArgs"
		}
		desc := goQuote(t.summary)
		switch {
		case t.image:
			fmt.Fprintf(&b, "\taddCapture[%s](s, %q, %s, wire.%s)\n", in, t.name, desc, t.method)
		case t.digest != "" && t.input == "":
			fmt.Fprintf(&b, "\taddSummarized[%s](s, %q, %s, wire.%s, %s)\n", t.output, t.name, desc, t.method, t.digest)
		case t.digest != "":
			fmt.Fprintf(&b, "\taddSummarizedIn[%s, %s](s, %q, %s, wire.%s, %s)\n", t.input, t.output, t.name, desc, t.method, t.digest)
		default:
			fmt.Fprintf(&b, "\taddForward[%s](s, %q, %s, wire.%s)\n", in, t.name, desc, t.method)
		}
	}
	b.WriteString("}\n")
	return []byte(b.String())
}

// goQuote renders a description as a Go string literal (kept readable as %q).
func goQuote(s string) string {
	return fmt.Sprintf("%q", s)
}
