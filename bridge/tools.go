// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

// Tool input shapes that stay local rather than using the wire DTO directly, because
// the MCP SDK derives each tool's input schema from the Go struct and the wire form
// would reflect badly: noArgs (empty input), and addFeatureArg, whose Args is a
// free-form object (each feature kind has its own schema, see list_feature_kinds — the
// dynamic LLM input boundary). They are wired in via mcp:input overrides in api/client,
// re-marshaled to the wire JSON by the generated registration. (The matrix-bearing
// assembly inputs do the same; see tools_assembly.go.)
type (
	noArgs        struct{}
	addFeatureArg struct {
		Kind string         `json:"kind"`
		Args map[string]any `json:"args"`
	}
)

//go:generate go run ../internal/mcpgen

// registerTools wires every MCP tool to its host method. The entire tool surface is
// GENERATED from the api/client mcp: annotations (the API is the single source of
// truth) — see internal/mcpgen and tools_generated.go. There are no hand-written tool
// registrations; regenerate with `go generate ./...`. (MCP resources, which are not
// method-backed tools, are registered separately from the server; see resources.go.)
func (s *Server) registerTools() {
	s.registerGeneratedTools()
}

// addSummarizedIn is [addSummarized] for a tool that takes typed input In: it forwards In to
// the host, returns a one-line digest as text, and keeps the full reply as structured
// content. Used for the per-sketch enumerations (entities/constraints/dimensions/profiles)
// whose JSON grows with the model — the model reads a digest, not a wall of JSON.
func addSummarizedIn[In any, Out any](s *Server, name, desc, method string, summarize func(Out) string) {
	s.addToolSafe(name, desc, method, func() {
		mcp.AddTool(s.mcp, &mcp.Tool{Name: name, Description: desc},
			func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
				resp, err := s.callMarshaled(method, in)
				if err != nil {
					return nil, nil, err
				}
				var out Out
				if err := json.Unmarshal(resp, &out); err != nil {
					return nil, nil, fmt.Errorf("%s: decode reply: %w", method, err)
				}
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summarize(out)}}}, json.RawMessage(resp), nil
			})
	})
}

// addForward registers a tool that marshals its typed input In and forwards it to
// the given host method, returning the host's JSON result as the tool's text content.
func addForward[In any](s *Server, name, desc, method string) {
	s.addToolSafe(name, desc, method, func() {
		mcp.AddTool(s.mcp, &mcp.Tool{Name: name, Description: desc},
			func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
				return s.forward(method, in)
			})
	})
}

// addToolSafe runs build() to register a typed tool, but recovers if the MCP SDK
// panics deriving the input schema — which it does for a recursive wire DTO it can't
// represent (e.g. a browser tree's self-referential nodes). The fallback registers
// the same tool with a permissive free-form-object input that still forwards to the
// host method, so every annotated method stays callable rather than crashing startup.
func (s *Server) addToolSafe(name, desc, method string, build func()) {
	defer func() {
		if recover() != nil {
			mcp.AddTool(s.mcp, &mcp.Tool{Name: name, Description: desc + " (input: free-form JSON object)"},
				func(_ context.Context, _ *mcp.CallToolRequest, in map[string]any) (*mcp.CallToolResult, any, error) {
					return s.forward(method, in)
				})
		}
	}()
	build()
}

// forward marshals in, calls the host method, and wraps the JSON reply as text content. A
// host error becomes a tool error, enriched with the request payload — the host message already
// names the method (the router prefixes it), so we add the offending arguments, which the host
// error lacks, to make an invalid command self-explanatory to the caller.
func (s *Server) forward(method string, in any) (*mcp.CallToolResult, any, error) {
	req, err := json.Marshal(in)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: encode request: %w", method, err)
	}
	resp, err := s.caller.Call(method, req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w (request: %s)", err, clipJSON(req))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(resp)}}}, nil, nil
}

// callMarshaled marshals in to JSON and calls the host method, returning the raw reply.
func (s *Server) callMarshaled(method string, in any) ([]byte, error) {
	req, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return s.caller.Call(method, req)
}

// clipJSON bounds a request payload echoed into an error so a large argument blob can't bloat
// the message.
func clipJSON(b []byte) string {
	const max = 240
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

// addSummarized registers a tool that, instead of returning the host's raw JSON, renders
// it through summarize into a short human-readable line as the text result while still
// returning the full reply as structured content. Used for list/inspect tools whose JSON
// is noisy (materials, appearances, themes) so a model reads a digest, not a wall of JSON.
func addSummarized[Out any](s *Server, name, desc, method string, summarize func(Out) string) {
	mcp.AddTool(s.mcp, &mcp.Tool{Name: name, Description: desc},
		func(_ context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			resp, err := s.caller.Call(method, nil)
			if err != nil {
				return nil, nil, err
			}
			var out Out
			if err := json.Unmarshal(resp, &out); err != nil {
				return nil, nil, fmt.Errorf("%s: decode reply: %w", method, err)
			}
			text := summarize(out)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, json.RawMessage(resp), nil
		})
}

// summarizeMaterials / summarizeAppearances / summarizeThemes / summarizeActiveTheme render a
// host reply as a one-line digest. List digests keep each item's id so a model can follow up
// with get_/assign_ by id.
func summarizeMaterials(r wire.ListMaterialsResult) string {
	items := make([]string, len(r.Materials))
	for i, m := range r.Materials {
		items[i] = fmt.Sprintf("%s [%s]", m.DisplayName, m.ID)
	}
	return countList("material", items)
}

func summarizeAppearances(r wire.ListAppearancesResult) string {
	items := make([]string, len(r.Appearances))
	for i, a := range r.Appearances {
		items[i] = fmt.Sprintf("%s [%s]", a.DisplayName, a.ID)
	}
	return countList("appearance", items)
}

func summarizeThemes(r wire.ListThemesResult) string {
	items := make([]string, len(r.Themes))
	for i, t := range r.Themes {
		if t.Active {
			items[i] = t.Name + " (active)"
		} else {
			items[i] = t.Name
		}
	}
	return countList("theme", items)
}

func summarizeActiveTheme(v wire.ThemeView) string {
	return fmt.Sprintf("Active theme: %s (%s), %d colors.", v.Name, v.Kind, len(v.Colors))
}

// countList renders "N noun(s): a, b, c." (or "No nouns." when empty).
func countList(noun string, items []string) string {
	if len(items) == 0 {
		return fmt.Sprintf("No %ss.", noun)
	}
	return fmt.Sprintf("%d %s(s): %s.", len(items), noun, strings.Join(items, ", "))
}
