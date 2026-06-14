// SPDX-License-Identifier: GPL-2.0-only

// Command mcptest exercises EVERY tool of a running oblikovati-mcp-bridge endpoint and
// prints a per-tool report. It first runs a realistic happy-path workflow (create a part,
// author and constrain a sketch, build a feature, a 3D sketch, tweak view/lighting, undo),
// chaining ids between calls so the core mutating tools get valid arguments; then it sweeps
// every remaining tool with a minimal argument object synthesized from the tool's own input
// schema, so the call still reaches the host (proving the endpoint is wired).
//
// Each tool is classified:
//   - PASS   : the bridge forwarded and the host returned a non-error result.
//   - REACH  : the bridge forwarded and the host returned a (semantic) error — the endpoint
//     is wired and responding, the synthesized args just weren't valid for the live
//     state (e.g. an id that doesn't exist).
//   - SCHEMA : the MCP client rejected the args against the tool's input schema before
//     sending (a gap in this harness's argument synthesis, not a bridge bug).
//   - FAIL   : a transport/protocol failure (a genuine bridge problem).
//   - MISS   : advertised but never called.
//
// Usage: mcptest [--url http://127.0.0.1:7800/mcp] [-v]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:7800/mcp", "MCP endpoint URL")
	verbose := flag.Bool("v", false, "print each tool's reply/error detail")
	flag.Parse()
	if err := run(*url, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "mcptest:", err)
		os.Exit(1)
	}
}

type outcome struct {
	status string // PASS | REACH | SCHEMA | FAIL | MISS
	detail string
}

type runner struct {
	ctx     context.Context
	cs      *mcp.ClientSession
	results map[string]outcome
	verbose bool
}

func run(url string, verbose bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcptest", Version: "0.2.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", url, err)
	}
	defer func() {
		if closeErr := cs.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "mcptest: close session:", closeErr)
		}
	}()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	fmt.Printf("connected: %d tools advertised\n\n", len(tools.Tools))

	r := &runner{ctx: ctx, cs: cs, results: map[string]outcome{}, verbose: verbose}
	r.workflow()
	r.sweep(tools.Tools)
	return r.report(tools.Tools)
}

// call invokes a tool, classifies the outcome, and returns the parsed reply for chaining.
func (r *runner) call(name string, args map[string]any) map[string]any {
	res, err := r.cs.CallTool(r.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		status := "FAIL"
		if strings.Contains(err.Error(), "invalid params") {
			status = "SCHEMA"
		}
		r.record(name, status, err.Error())
		return nil
	}
	text := firstText(res)
	if res.IsError {
		r.record(name, "REACH", text)
		return nil
	}
	r.record(name, "PASS", text)
	var parsed map[string]any
	_ = json.Unmarshal([]byte(text), &parsed)
	return parsed
}

// record keeps the strongest status seen for a tool (PASS > REACH > SCHEMA > FAIL).
func (r *runner) record(name, status, detail string) {
	rank := map[string]int{"FAIL": 0, "MISS": 0, "SCHEMA": 1, "REACH": 2, "PASS": 3}
	if prev, ok := r.results[name]; ok && rank[prev.status] >= rank[status] {
		return
	}
	r.results[name] = outcome{status, clip(detail, 150)}
	if r.verbose {
		fmt.Printf("  %-32s %-6s %s\n", name, status, clip(detail, 200))
	}
}

// ---- the happy-path workflow (ids chained between steps) -------------------

func (r *runner) workflow() {
	r.documentAndParams()
	r.sketchAndConstraints()
	r.featuresFromSketch()
	r.sketch3D()
	r.viewLightingEnvironment()
	r.assetsAndCommands()
	r.session()
}

func (r *runner) documentAndParams() {
	r.call("list_documents", nil)
	r.call("create_document", map[string]any{"type": "part", "name": "mcptest"})
	r.call("activate_document", map[string]any{"id": 1})
	r.call("create_document", map[string]any{"type": "part", "name": "mcptest2"})
	r.call("get_model_tree", nil)
	r.call("get_selection", nil)
	r.call("list_parameters", nil)
	r.call("add_parameter", map[string]any{"name": "w", "expression": "40 mm"})
	r.call("get_parameter", map[string]any{"name": "w"})
	r.call("set_parameter", map[string]any{"name": "w", "expression": "50 mm"})
}

func (r *runner) sketchAndConstraints() {
	r.call("create_sketch", map[string]any{"plane": "XY"})
	r.call("list_sketches", nil)
	r.call("get_sketch", map[string]any{"sketchIndex": 0})
	r.call("edit_sketch", map[string]any{"sketchIndex": 0})
	r.call("sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"})
	line := r.call("add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "line", "points": [][]float64{{6, 6}, {8, 6}},
	})
	r.call("add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "circle", "points": [][]float64{{2, 2}}, "radius": "5 mm",
	})
	r.call("add_sketch_text", map[string]any{"sketchIndex": 0, "anchor": []float64{1, 9}, "text": "OBK", "height": "3 mm"})
	r.call("list_sketch_entities", map[string]any{"sketchIndex": 0})

	ends := idList(line, "pointIds")
	if len(ends) >= 2 {
		r.call("add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "horizontal", "entities": ends[:2]})
		r.call("add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": ends[:2], "expression": "20 mm"})
		r.call("delete_sketch_constraint", map[string]any{"sketchIndex": 0, "constraintIndex": 0})
		r.call("drive_sketch_dimension", map[string]any{"sketchIndex": 0, "dimensionIndex": 0, "expression": "25 mm"})
	}
	r.call("offset_sketch", map[string]any{"sketchIndex": 0, "entity": ends0(line), "distance": "2 mm"})
	r.call("list_sketch_constraints", map[string]any{"sketchIndex": 0})
	r.call("list_sketch_dimensions", map[string]any{"sketchIndex": 0})
	r.call("get_sketch_constraint_status", map[string]any{"sketchIndex": 0})
	r.call("auto_dimension_sketch", map[string]any{"sketchIndex": 0})
	r.call("solve_sketch", map[string]any{"sketchIndex": 0})
	r.call("set_sketch_property", map[string]any{"sketchIndex": 0, "property": "name", "value": "Base"})
	r.call("exit_sketch", map[string]any{"sketchIndex": 0})
}

func (r *runner) featuresFromSketch() {
	r.call("list_sketch_profiles", map[string]any{"sketchIndex": 0})
	r.call("list_feature_kinds", nil)
	r.call("add_feature", map[string]any{
		"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"},
	})
	r.call("get_physical_properties", nil)
	r.call("get_reference_keys", nil)
}

func (r *runner) sketch3D() {
	r.call("create_sketch3d", map[string]any{"name": "Sketch3D1"})
	r.call("list_sketches3d", nil)
	r.call("get_sketch3d", map[string]any{"sketchIndex": 0})
	r.call("edit_sketch3d", map[string]any{"sketchIndex": 0})
	line := r.call("add_sketch3d_entity", map[string]any{
		"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0, 0}, {10, 0, 0}},
	})
	r.call("list_sketch3d_entities", map[string]any{"sketchIndex": 0})
	r.call("list_sketch3d_constraints", map[string]any{"sketchIndex": 0})
	r.call("list_sketch3d_dimensions", map[string]any{"sketchIndex": 0})
	r.call("list_sketch3d_profiles", map[string]any{"sketchIndex": 0})
	r.call("list_sketch3d_paths", map[string]any{"sketchIndex": 0})
	r.call("get_sketch3d_constraint_status", map[string]any{"sketchIndex": 0})
	r.call("solve_sketch3d", map[string]any{"sketchIndex": 0})
	r.call("set_sketch3d_property", map[string]any{"sketchIndex": 0, "property": "name", "value": "S3D"})
	_ = line
	r.call("exit_sketch3d", map[string]any{"sketchIndex": 0})
}

func (r *runner) viewLightingEnvironment() {
	r.call("get_display_mode", nil)
	modes := r.call("list_display_modes", nil)
	if m, ok := firstNum(modes, "modes", "mode"); ok {
		r.call("set_display_mode", map[string]any{"mode": m})
	}
	if sh := r.call("get_shadows", nil); sh != nil {
		r.call("set_shadows", sh) // echo the current settings back (valid shape)
	}
	r.call("get_lighting_style", nil)
	styles := r.call("list_lighting_styles", nil)
	if name := firstStr(styles, "styles", "name"); name != "" {
		r.call("set_lighting_style", map[string]any{"name": name})
	}
	r.call("list_lights", nil)
	r.call("get_environment", nil)
	r.call("set_environment", map[string]any{"preset": "Studio", "rotation": 0, "intensity": 1.0, "showImage": false})
	r.call("list_environment_presets", nil)
}

func (r *runner) assetsAndCommands() {
	mats := r.call("list_materials", nil)
	if id := firstStr(mats, "materials", "id"); id != "" {
		r.call("get_material", map[string]any{"id": id})
		r.call("assign_material", map[string]any{"materialId": id})
		r.call("create_material", map[string]any{"baseId": id, "name": "mcptest-mat"})
	}
	aps := r.call("list_appearances", nil)
	if id := firstStr(aps, "appearances", "id"); id != "" {
		r.call("get_appearance", map[string]any{"id": id})
		r.call("assign_appearance", map[string]any{"scope": "part", "appearanceId": id})
		r.call("create_appearance", map[string]any{"baseId": id, "name": "mcptest-appr"})
	}
	cmds := r.call("list_commands", nil)
	if id := firstStr(cmds, "commands", "id"); id != "" {
		r.call("execute_command", map[string]any{"id": id})
	}
	r.call("get_ribbon", nil)
	r.call("create_command", map[string]any{"id": "mcptest.btn", "displayName": "MCP Test"})
	r.call("list_work_planes", nil)
	r.call("get_active_theme", nil)
	r.call("list_themes", nil)
}

func (r *runner) session() {
	r.call("list_client_graphics", nil)
	r.call("clear_interaction_graphics", nil)
	r.call("get_undo_state", nil)
	r.call("undo", nil)
	r.call("redo", nil)
}

// ---- schema-driven sweep of anything the workflow skipped ------------------

func (r *runner) sweep(tools []*mcp.Tool) {
	for _, tool := range tools {
		if _, done := r.results[tool.Name]; done {
			continue
		}
		r.call(tool.Name, synthArgs(tool.InputSchema))
	}
}

// synthArgs builds a minimal argument object satisfying a tool's JSON input schema, so the
// call passes client-side validation and reaches the host. Required fields are filled with
// type-appropriate zero values (arrays get one synthesized element to satisfy minItems).
func synthArgs(schema any) map[string]any {
	node := decodeSchema(schema)
	v, _ := synthValue(node).(map[string]any)
	return v
}

type schemaNode struct {
	TypeRaw     json.RawMessage        `json:"type"` // string OR []string (a nullable/union type)
	Required    []string               `json:"required"`
	Properties  map[string]*schemaNode `json:"properties"`
	Items       *schemaNode            `json:"items"`
	PrefixItems []*schemaNode          `json:"prefixItems"`
	MinItems    int                    `json:"minItems"`
}

func decodeSchema(schema any) *schemaNode {
	raw, err := json.Marshal(schema)
	if err != nil {
		return &schemaNode{}
	}
	var n schemaNode
	_ = json.Unmarshal(raw, &n)
	return &n
}

// kind resolves the node's effective JSON type, tolerating a union "type" array (picks the
// first concrete type) and inferring object/array from the presence of properties/items when
// "type" is absent.
func (n *schemaNode) kind() string {
	for _, t := range n.declaredTypes() {
		switch t {
		case "object", "array", "string", "integer", "number", "boolean":
			return t
		}
	}
	if len(n.Properties) > 0 || len(n.Required) > 0 {
		return "object"
	}
	if n.Items != nil || len(n.PrefixItems) > 0 {
		return "array"
	}
	return "object"
}

func (n *schemaNode) declaredTypes() []string {
	if len(n.TypeRaw) == 0 {
		return nil
	}
	var one string
	if json.Unmarshal(n.TypeRaw, &one) == nil {
		return []string{one}
	}
	var many []string
	_ = json.Unmarshal(n.TypeRaw, &many)
	return many
}

func synthValue(n *schemaNode) any {
	switch n.kind() {
	case "object":
		m := map[string]any{}
		for _, req := range n.Required {
			if p := n.Properties[req]; p != nil {
				m[req] = synthValue(p)
			} else {
				m[req] = ""
			}
		}
		return m
	case "array":
		return synthArray(n)
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		return ""
	}
}

// synthArray builds a non-empty array satisfying a tuple (prefixItems) or a homogeneous
// (items + minItems) array schema, so minItems-constrained fields like a 2D point pass.
func synthArray(n *schemaNode) []any {
	if len(n.PrefixItems) > 0 {
		out := make([]any, len(n.PrefixItems))
		for i, p := range n.PrefixItems {
			out[i] = synthValue(p)
		}
		return out
	}
	count := n.MinItems
	if count < 1 {
		count = 1
	}
	out := make([]any, count)
	for i := range out {
		if n.Items != nil {
			out[i] = synthValue(n.Items)
		} else {
			out[i] = 0
		}
	}
	return out
}

// ---- reporting -------------------------------------------------------------

func (r *runner) report(tools []*mcp.Tool) error {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	sort.Strings(names)
	counts := map[string]int{}
	fmt.Printf("%-34s %s\n", "TOOL", "RESULT")
	for _, name := range names {
		o, ok := r.results[name]
		if !ok {
			o = outcome{"MISS", "not called"}
		}
		counts[o.status]++
		fmt.Printf("%-34s %-6s %s\n", name, o.status, o.detail)
	}
	fmt.Printf("\nsummary: %d advertised | %d PASS | %d REACH | %d SCHEMA | %d FAIL | %d MISS\n",
		len(names), counts["PASS"], counts["REACH"], counts["SCHEMA"], counts["FAIL"], counts["MISS"])
	if counts["FAIL"] > 0 {
		return fmt.Errorf("%d tool(s) had a transport/bridge failure", counts["FAIL"])
	}
	return nil
}

// ---- helpers ---------------------------------------------------------------

func firstText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func idList(reply map[string]any, key string) []uint64 {
	raw, ok := reply[key].([]any)
	if !ok {
		return nil
	}
	out := make([]uint64, 0, len(raw))
	for _, v := range raw {
		if f, ok := v.(float64); ok {
			out = append(out, uint64(f))
		}
	}
	return out
}

func ends0(reply map[string]any) uint64 {
	if ids := idList(reply, "pointIds"); len(ids) > 0 {
		return ids[0]
	}
	return 0
}

func firstObj(reply map[string]any, listKey string) (map[string]any, bool) {
	arr, ok := reply[listKey].([]any)
	if !ok || len(arr) == 0 {
		return nil, false
	}
	obj, ok := arr[0].(map[string]any)
	return obj, ok
}

func firstStr(reply map[string]any, listKey, field string) string {
	if obj, ok := firstObj(reply, listKey); ok {
		if s, ok := obj[field].(string); ok {
			return s
		}
	}
	return ""
}

func firstNum(reply map[string]any, listKey, field string) (float64, bool) {
	if obj, ok := firstObj(reply, listKey); ok {
		if f, ok := obj[field].(float64); ok {
			return f, true
		}
	}
	return 0, false
}

func clip(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
