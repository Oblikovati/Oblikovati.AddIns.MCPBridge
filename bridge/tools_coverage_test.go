// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

// registeredToolNames returns the names of every tool the bridge advertises.
func registeredToolNames(t *testing.T) []string {
	t.Helper()
	cs := connect(t, &fakeHost{reply: []byte("{}")})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, len(res.Tools))
	for i, tool := range res.Tools {
		names[i] = tool.Name
	}
	return names
}

// TestNoDuplicateToolNames guards the MCP contract that every tool name is unique — a
// duplicate registration would silently shadow a tool for the model.
func TestNoDuplicateToolNames(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range registeredToolNames(t) {
		if seen[n] {
			t.Errorf("duplicate tool name %q", n)
		}
		seen[n] = true
	}
}

// TestNewlyWiredToolsRegistered pins the domains added when the bridge was brought back up to
// the host's API surface: 2D + 3D sketch authoring, view, lighting/environment, client &
// interaction graphics, undo/redo, and reference keys. A representative tool from each group
// must be present (a dropped registration regresses this).
func TestNewlyWiredToolsRegistered(t *testing.T) {
	got := registeredToolNames(t)
	want := []string{
		// 2D sketch
		"list_sketches", "get_sketch", "list_sketch_entities", "list_sketch_constraints",
		"list_sketch_dimensions", "list_sketch_profiles", "edit_sketch", "solve_sketch",
		"add_sketch_entity", "transform_sketch", "offset_sketch", "add_sketch_pattern",
		"add_sketch_text", "add_fill_region", "add_sketch_image", "project_geometry",
		"add_sketch_constraint", "delete_sketch_constraint", "add_sketch_dimension",
		"drive_sketch_dimension", "auto_dimension_sketch", "set_sketch_property",
		"get_sketch_constraint_status", "exit_sketch", "delete_sketch",
		// 3D sketch
		"list_sketches3d", "get_sketch3d", "create_sketch3d", "edit_sketch3d",
		"add_sketch3d_entity", "add_sketch3d_constraint", "add_sketch3d_dimension",
		"list_sketch3d_paths", "include_sketch3d_geometry", "include_2d_sketch_in_3d",
		"add_sketch3d_surface_curve", "transform_sketch3d",
		// view / lighting / environment
		"get_display_mode", "set_display_mode", "list_display_modes", "get_shadows",
		"set_shadows", "get_lighting_style", "list_lighting_styles", "set_lighting_style",
		"list_lights", "add_light", "set_light", "get_environment", "set_environment",
		"list_environment_presets", "load_environment_image",
		// graphics overlays
		"set_client_graphics", "list_client_graphics", "delete_client_graphics",
		"set_client_graphics_visible", "update_interaction_graphics", "clear_interaction_graphics",
		// session
		"undo", "redo", "get_undo_state", "get_reference_keys",
	}
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("tool %q not registered", name)
		}
	}
}

// TestAssemblyToolsRegistered pins the assembly occurrence surface (M11) exposed to MCP — read
// the tree and place/copy/transform/ground/suppress/replace/remove components. A dropped
// registration regresses this.
func TestAssemblyToolsRegistered(t *testing.T) {
	got := registeredToolNames(t)
	want := []string{
		"list_occurrences", "place_component", "place_component_copy", "transform_occurrence",
		"ground_occurrence", "suppress_occurrence", "replace_occurrence", "remove_occurrence",
	}
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("assembly tool %q not registered", name)
		}
	}
}

// TestPlaceComponentForwards spot-checks place_component forwards its typed args (document id,
// name, 4×4 transform) to the assembly.place host method.
func TestPlaceComponentForwards(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"occurrence":{"id":7,"name":"widget:1","transform":[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1]}}`)}
	cs := connect(t, host)
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "place_component",
		Arguments: map[string]any{"document": 3, "name": "widget:1", "transform": []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if host.lastMethod != wire.MethodAssemblyPlace {
		t.Fatalf("method = %q, want %q", host.lastMethod, wire.MethodAssemblyPlace)
	}
	if !strings.Contains(string(host.lastReq), `"document":3`) || !strings.Contains(string(host.lastReq), `"widget:1"`) {
		t.Fatalf("forwarded req = %s, want document 3 + name widget:1", host.lastReq)
	}
}

// TestNewSketchToolForwards spot-checks a 2D sketch tool forwards to the right host method
// with its typed args.
func TestNewSketchToolForwards(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"index":2,"name":"Sketch1","plane":"XY","entityCount":4,"dof":0}`)}
	cs := connect(t, host)
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_sketch",
		Arguments: map[string]any{"sketchIndex": 2},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if host.lastMethod != wire.MethodSketchGet {
		t.Fatalf("method = %q, want %q", host.lastMethod, wire.MethodSketchGet)
	}
	if !strings.Contains(string(host.lastReq), `"sketchIndex":2`) {
		t.Fatalf("forwarded req = %s, want sketchIndex 2", host.lastReq)
	}
}

// TestSketchEntitiesDigest checks the per-sketch entity enumeration renders a token-efficient
// digest while keeping the full reply as structured content.
func TestSketchEntitiesDigest(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"entities":[{"index":0,"id":11,"kind":"line"},{"index":1,"id":12,"kind":"circle","construction":true}]}`)}
	cs := connect(t, host)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_sketch_entities",
		Arguments: map[string]any{"sketchIndex": 0},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := firstText(t, res)
	if !strings.Contains(text, "2 entity(s)") || !strings.Contains(text, "line id=11") || !strings.Contains(text, "construction") {
		t.Errorf("digest = %q, want a count + entity list with the construction tag", text)
	}
	if res.StructuredContent == nil {
		t.Error("entity digest should still carry the full reply as structured content")
	}
}

// TestSceneToolForwards spot-checks a scene tool (undo) reaches the transaction method.
func TestSceneToolForwards(t *testing.T) {
	host := &fakeHost{reply: []byte(`{"canUndo":false,"canRedo":true}`)}
	cs := connect(t, host)
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "undo"}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if host.lastMethod != wire.MethodTransactionUndo {
		t.Fatalf("method = %q, want %q", host.lastMethod, wire.MethodTransactionUndo)
	}
}
