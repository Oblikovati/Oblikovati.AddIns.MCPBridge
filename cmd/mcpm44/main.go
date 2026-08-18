// SPDX-License-Identifier: GPL-2.0-only

// Command mcpm44 is a live driver that stresses the M44 Inventor-parity API tranche
// against a running oblikovati-mcp-bridge: the assembly options/virtual/BOM/DOF/joint
// surfaces and the drawing sheet-authoring/view/dimension/annotation surfaces. It drives
// the same path an LLM client takes and writes PNG screenshots of the rendered drawing
// sheet so the result can be checked visually (CLAUDE.md live-test rule).
//
// Usage: mcpm44 [--url http://127.0.0.1:7800/mcp] [--out /dir/for/screenshots]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	out := flag.String("out", ".", "directory for screenshots")
	flag.Parse()
	if err := run(*url, *out); err != nil {
		fmt.Fprintln(os.Stderr, "mcpm44:", err)
		os.Exit(1)
	}
}

// d drives one MCP session and records pass/warn lines for a final report.
type d struct {
	ctx  context.Context
	cs   *mcp.ClientSession
	out  string
	fail int
}

func run(url, out string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpm44", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer cs.Close()
	dr := &d{ctx: ctx, cs: cs, out: out}
	dr.assembly()
	dr.drawing()
	fmt.Printf("\n==== M44 live driver done: %d warnings ====\n", dr.fail)
	return nil
}

// call invokes a tool and returns its text result; a host error is returned as err.
func (dr *d) call(tool string, args any) (string, error) {
	res, err := dr.cs.CallTool(dr.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			b.WriteString(t.Text)
		}
	}
	if res.IsError {
		return b.String(), fmt.Errorf("tool %s error: %s", tool, b.String())
	}
	return b.String(), nil
}

// step runs a tool, logs PASS/WARN, and returns the raw text (empty on error).
func (dr *d) step(label, tool string, args any) string {
	txt, err := dr.call(tool, args)
	if err != nil {
		dr.fail++
		fmt.Printf("  WARN %-28s %v\n", label, err)
		return ""
	}
	fmt.Printf("  PASS %-28s %s\n", label, clip(txt, 160))
	return txt
}

// openDoc creates a document of the given type/name and activates it; if the name is
// already open it finds and activates the existing one. Returns its id. Calls that act on
// the active document (assembly options, base views) need the new document to BE active.
func (dr *d) openDoc(docType, name string) uint64 {
	txt, err := dr.call("create_document", map[string]any{"type": docType, "name": name})
	id := jsonUint(txt, "id")
	if err != nil {
		id = docIDByName(dr.step("list_documents", "list_documents", map[string]any{}), name)
	}
	if id == 0 {
		dr.fail++
		fmt.Printf("  WARN %-28s could not open %s %q\n", "openDoc", docType, name)
		return 0
	}
	dr.step("open+activate "+docType, "activate_document", map[string]any{"id": id})
	return id
}

func clip(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
