// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"

	"oblikovati.org/app"
)

// This is the end-to-end validation suite: it drives every tool/feature over the MCP tool
// layer against the REAL router + live model (in-process, via routerHost), asserting each
// behaves. It is the deterministic, CI-friendly companion to the live cmd/mcptest harness.
// Tools that need elaborate live state (topology refs, every feature's prerequisites) are
// asserted "reachable + non-panicking"; the core authoring path is asserted for real success.

// freshPart returns an MCP client over a brand-new empty part — the clean slate the authoring
// tests build on (sketchIndex 0 after the first create_sketch).
func freshPart(t *testing.T) *mcp.ClientSession {
	t.Helper()
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	return e2eClient(t, s)
}

// mustReach calls a tool and fails only on a transport/schema error or a recovered panic; a
// clean domain error (IsError without "panic") is acceptable — the endpoint is wired and the
// kernel did not crash. Returns the reply text.
func mustReach(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: transport/schema error: %v", name, err)
	}
	text := firstText(t, res)
	if res.IsError && strings.Contains(text, "panic") {
		t.Fatalf("%s PANICKED (kernel bug): %s", name, text)
	}
	return text
}

// callStructured calls a summarized tool (whose text content is a digest) and decodes its
// full JSON reply from the structured content into v.
func callStructured(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, v any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s tool error: %s", name, firstText(t, res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s: marshal structured content: %v", name, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: decode structured %q: %v", name, raw, err)
	}
}

// idsOf extracts entityIds (the defining-point ids) from an add_sketch_entity reply.
func idsOf(t *testing.T, cs *mcp.ClientSession, args map[string]any) []uint64 {
	t.Helper()
	var r struct {
		EntityID  uint64   `json:"entityId"`
		PointIDs  []uint64 `json:"pointIds"`
		EntityIDs []uint64 `json:"entityIds"`
	}
	callJSON(t, cs, "add_sketch_entity", args, &r)
	if len(r.PointIDs) > 0 {
		return append([]uint64{r.EntityID}, r.PointIDs...)
	}
	return append([]uint64{r.EntityID}, r.EntityIDs...)
}

// TestE2EAllSketchEntityKinds authors every 2D entity kind and variant the host supports and
// asserts each is created (the sketch's entity count grows monotonically).
func TestE2EAllSketchEntityKinds(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	entities := []map[string]any{
		{"kind": "line", "points": [][]float64{{0, 0}, {4, 0}}},
		{"kind": "point", "points": [][]float64{{1, 1}}},
		{"kind": "circle", "points": [][]float64{{2, 2}}, "radius": "1 cm"},
		{"kind": "circle", "variant": "threePoint", "points": [][]float64{{0, 0}, {2, 0}, {1, 1}}},
		{"kind": "arc", "points": [][]float64{{0, 0}, {2, 0}, {0, 2}}, "ccw": true},
		{"kind": "arc", "variant": "threePoint", "points": [][]float64{{0, 0}, {1, 1}, {2, 0}}},
		{"kind": "rectangle", "points": [][]float64{{0, 0}, {4, 3}}},
		{"kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2, 1}}},
		{"kind": "rectangle", "variant": "threePoint", "points": [][]float64{{0, 0}, {4, 0}, {4, 3}}},
		{"kind": "polygon", "points": [][]float64{{0, 0}, {2, 0}}, "sides": 6},
		{"kind": "slot", "points": [][]float64{{0, 0}, {4, 0}}, "width": "1 cm"},
		{"kind": "slot", "variant": "arc", "points": [][]float64{{0, 0}, {2, 2}, {4, 0}}, "width": "1 cm", "ccw": true},
		{"kind": "ellipse", "points": [][]float64{{0, 0}}, "axis": []float64{1, 0}, "majorRadius": "3 cm", "minorRadius": "1 cm"},
		{"kind": "ellipticalArc", "points": [][]float64{{0, 0}}, "axis": []float64{1, 0}, "majorRadius": "3 cm", "minorRadius": "1 cm", "startAngle": "0 deg", "endAngle": "90 deg"},
		{"kind": "spline", "points": [][]float64{{0, 0}, {1, 1}, {2, 0}}},
		{"kind": "spline", "variant": "controlPoint", "points": [][]float64{{0, 0}, {1, 1}, {2, 0}}},
	}
	for _, e := range entities {
		e["sketchIndex"] = 0
		var r struct {
			Kind string `json:"kind"`
		}
		callJSON(t, cs, "add_sketch_entity", e, &r) // callJSON fails on any tool error
	}
	callJSON(t, cs, "add_sketch_text", map[string]any{"sketchIndex": 0, "anchor": []float64{1, 9}, "text": "OBK", "height": "3 mm"}, nil)

	var ents struct {
		Entities []json.RawMessage `json:"entities"`
	}
	callStructured(t, cs, "list_sketch_entities", map[string]any{"sketchIndex": 0}, &ents)
	if len(ents.Entities) < len(entities) {
		t.Fatalf("sketch has %d entities, want >= %d authored kinds", len(ents.Entities), len(entities))
	}
}

// TestE2ESketchConstraintsAndDimensions adds constraints (each requiring real entity ids) and
// a dimension, then drives and deletes them — the constrain/dimension surface end to end.
func TestE2ESketchConstraintsAndDimensions(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	a := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0}, {4, 0}}})
	b := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 1}, {4, 2}}})
	lineA, pA0, pA1 := a[0], a[1], a[2]
	lineB, pB0 := b[0], b[1]

	constraints := []map[string]any{
		{"kind": "horizontal", "entities": []uint64{pA0, pA1}},
		{"kind": "parallel", "entities": []uint64{lineA, lineB}},
		{"kind": "coincident", "entities": []uint64{pA1, pB0}},
	}
	for _, c := range constraints {
		c["sketchIndex"] = 0
		callJSON(t, cs, "add_sketch_constraint", c, nil)
	}

	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "distance", "entities": []uint64{pA0, pA1}, "expression": "4 cm",
	}, nil)
	callJSON(t, cs, "drive_sketch_dimension", map[string]any{"sketchIndex": 0, "dimensionIndex": 0, "expression": "5 cm"}, nil)
	callJSON(t, cs, "delete_sketch_constraint", map[string]any{"sketchIndex": 0, "constraintIndex": 0}, nil)

	var dims struct {
		Dimensions []json.RawMessage `json:"dimensions"`
	}
	callStructured(t, cs, "list_sketch_dimensions", map[string]any{"sketchIndex": 0}, &dims)
	if len(dims.Dimensions) != 1 {
		t.Fatalf("dimensions = %d, want 1", len(dims.Dimensions))
	}
	callJSON(t, cs, "auto_dimension_sketch", map[string]any{"sketchIndex": 0}, nil)
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, nil)
}

// TestE2ESketchTransformsPatternsOffset exercises move/copy/rotate/mirror, both patterns, and
// offset with real entity ids.
func TestE2ESketchTransformsPatternsOffset(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	a := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0}, {4, 0}}})
	b := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 5}, {4, 5}}})
	lineA, lineB := a[0], b[0]

	mustReach(t, cs, "transform_sketch", map[string]any{"sketchIndex": 0, "op": "move", "entities": []uint64{lineA}, "vector": []float64{1, 1}})
	mustReach(t, cs, "transform_sketch", map[string]any{"sketchIndex": 0, "op": "copy", "entities": []uint64{lineA}, "vector": []float64{0, 2}})
	mustReach(t, cs, "transform_sketch", map[string]any{"sketchIndex": 0, "op": "rotate", "entities": []uint64{lineA}, "center": []float64{0, 0}, "angle": "30 deg"})
	mustReach(t, cs, "transform_sketch", map[string]any{"sketchIndex": 0, "op": "mirror", "entities": []uint64{lineA}, "mirrorLine": lineB})
	mustReach(t, cs, "add_sketch_pattern", map[string]any{"sketchIndex": 0, "kind": "rectangular", "entities": []uint64{lineA}, "count1": 2, "count2": 1, "spacing1": "2 cm", "spacing2": "2 cm"})
	mustReach(t, cs, "add_sketch_pattern", map[string]any{"sketchIndex": 0, "kind": "circular", "entities": []uint64{lineA}, "center": []float64{0, 0}, "count1": 4, "angle": "360 deg"})
	mustReach(t, cs, "offset_sketch", map[string]any{"sketchIndex": 0, "entity": lineA, "distance": "1 cm"})
}

// Per-feature deep validation (extrude + the subtractive family) and the registry drift guard
// live in e2e_features_test.go.

// TestE2EWorkPlanesView covers work planes, display mode, shadows, and the 3D-sketch surface.
func TestE2EWorkPlanesView(t *testing.T) {
	cs := freshPart(t)
	var planes struct {
		Planes []struct {
			Ref string `json:"ref"`
		} `json:"planes"`
	}
	callJSON(t, cs, "list_work_planes", nil, &planes)
	if len(planes.Planes) < 3 {
		t.Fatalf("origin planes = %d, want >= 3", len(planes.Planes))
	}
	var plane struct {
		Healthy bool `json:"healthy"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{planes.Planes[0].Ref}, "offset": "2 cm"}, &plane)
	if !plane.Healthy {
		t.Errorf("offset work plane not healthy: %+v", plane)
	}

	mustReach(t, cs, "get_display_mode", nil)
	mustReach(t, cs, "get_shadows", nil)

	callJSON(t, cs, "create_sketch3d", map[string]any{"name": "S3D"}, nil)
	mustReach(t, cs, "add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0, 0}, {5, 0, 0}}})
	callJSON(t, cs, "solve_sketch3d", map[string]any{"sketchIndex": 0}, nil)
}

// TestE2ETransactionsUndoRedo validates the transaction surface is wired over the bridge:
// state reports a valid cursor and, when an edit is undoable, the undo→redo round trip holds.
// (Deep undo/redo correctness is covered by the router's transaction_test.go.)
func TestE2ETransactionsUndoRedo(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "3 cm"}, nil)
	var st wire.UndoState
	callJSON(t, cs, "get_undo_state", nil, &st)
	if st.CanUndo {
		callJSON(t, cs, "undo", nil, &st)
		if !st.CanRedo {
			t.Error("after undo, redo should be available")
		}
		callJSON(t, cs, "redo", nil, &st)
	} else {
		mustReach(t, cs, "undo", nil) // still callable end to end (a no-op returns a clean error)
		mustReach(t, cs, "redo", nil)
	}
}

// TestE2EInvalidCommandsGiveDetailedErrors: invalid commands return a detailed, method-named
// tool error (not a crash), so a driver can diagnose what it sent wrong.
func TestE2EInvalidCommandsGiveDetailedErrors(t *testing.T) {
	cs := freshPart(t)
	cases := []struct {
		name string
		args map[string]any
		want string // substring the error must contain
	}{
		{"get_sketch", map[string]any{"sketchIndex": 99}, "sketch"},
		{"activate_document", map[string]any{"id": 9999}, "document"},
		{"add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "banana", "points": [][]float64{{0, 0}}}, "kind"},
	}
	for _, c := range cases {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: c.name, Arguments: c.args})
		if err != nil {
			t.Fatalf("%s: transport error: %v", c.name, err)
		}
		if !res.IsError {
			t.Errorf("%s with bad args should be a tool error", c.name)
			continue
		}
		msg := firstText(t, res)
		if strings.Contains(msg, "panic") {
			t.Errorf("%s should be a clean error, not a panic: %s", c.name, msg)
		}
		if !strings.Contains(strings.ToLower(msg), c.want) {
			t.Errorf("%s error %q should mention %q", c.name, msg, c.want)
		}
	}
}

// TestE2ETailLogsReflectsOperations: after driving ops over the bridge, tail_logs shows the
// trace — successes timed, failures recorded — proving the real-time diagnostics surface.
func TestE2ETailLogsReflectsOperations(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	_, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{ // a deliberate failure to trace
		Name: "get_sketch", Arguments: map[string]any{"sketchIndex": 77},
	})

	var logs wire.LogsResult
	callJSON(t, cs, "tail_logs", map[string]any{}, &logs)
	if len(logs.Records) == 0 {
		t.Fatal("tail_logs returned no records after driving operations")
	}
	var sawCreate, sawFail bool
	for _, r := range logs.Records {
		if r.Method == "sketch.create" && r.OK {
			sawCreate = true
		}
		if r.Method == "sketch.get" && !r.OK && r.Error != "" {
			sawFail = true
		}
		if r.Method == "logs.tail" {
			t.Error("logs.tail must not appear in its own trace")
		}
	}
	if !sawCreate || !sawFail {
		t.Errorf("trace missing expected entries: sawCreate=%v sawFail=%v", sawCreate, sawFail)
	}
}
