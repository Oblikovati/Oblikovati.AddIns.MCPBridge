// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"

	"oblikovati.org/app"
)

// This file validates EVERY assembly constraint type end to end over the bridge (M12-F01).
// Each type is added through its MCP tool against real component geometry (box faces/edges,
// a drilled cylinder), the assembly is solved, and the free component's remaining degrees of
// freedom are asserted — the same DOF the engine's unit tests check, now proven over the
// MCP → router → solver path. Positioning correctness is shown by TestEndToEndAssembly-
// MateConstraint; here the DOF is the runtime contract for the whole family.

// boxKeys holds reference keys for a box component's faces (by outward normal) and one edge.
type boxKeys struct{ topZ, posX, posY, edge string }

// readBoxKeys builds a 40×30×20 mm box in the active part and returns its face keys (picked
// by representative-point extreme) and one straight edge key (a definition-space axis).
func readBoxKeys(t *testing.T, cs *mcp.ClientSession) boxKeys {
	t.Helper()
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"},
	}, nil)
	return boxKeysFromTopology(t, cs)
}

// boxKeysFromTopology classifies the active body's faces by representative point and takes
// one edge.
func boxKeysFromTopology(t *testing.T, cs *mcp.ClientSession) boxKeys {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
			Edges []struct {
				Key string `json:"key"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 || len(rk.Bodies[0].Faces) == 0 || len(rk.Bodies[0].Edges) == 0 {
		t.Fatal("get_reference_keys returned no box topology")
	}
	var k boxKeys
	hiZ, hiX, hiY := math.Inf(-1), math.Inf(-1), math.Inf(-1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) != 3 {
			continue
		}
		if f.Point[2] > hiZ {
			k.topZ, hiZ = f.Key, f.Point[2]
		}
		if f.Point[0] > hiX {
			k.posX, hiX = f.Key, f.Point[0]
		}
		if f.Point[1] > hiY {
			k.posY, hiY = f.Key, f.Point[1]
		}
	}
	k.edge = rk.Bodies[0].Edges[0].Key
	return k
}

// twoBoxes builds a box part and an assembly holding a grounded and a free instance, and
// returns the client, the grounded/free occurrence ids, and the shared box keys.
func twoBoxes(t *testing.T) (cs *mcp.ClientSession, grounded, free uint64, k boxKeys) {
	t.Helper()
	cs = e2eClient(t, app.NewSession())
	var widget, asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "box.obk"}, &widget)
	k = readBoxKeys(t, cs)
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var first, second wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{"document": widget.ID, "name": "box:1", "transform": identityCells}, &first)
	callJSON(t, cs, "place_component_copy", map[string]any{"source": first.Occurrence.ID, "name": "box:2", "transform": identityCells}, &second)
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": first.Occurrence.ID, "grounded": true}, nil)
	return cs, first.Occurrence.ID, second.Occurrence.ID, k
}

// geomRef builds a constraint geometry reference (occurrence id + entity reference key).
func geomRef(occ uint64, key string) map[string]any {
	return map[string]any{"occurrence": occ, "entity": key}
}

// addConstraint calls a constraint tool, solves the assembly, and returns the new
// constraint plus the free component's remaining DOF.
func addConstraint(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any, free uint64) (wire.ConstraintResult, wire.AssemblyHealthResult) {
	t.Helper()
	var added wire.ConstraintResult
	callJSON(t, cs, tool, args, &added)
	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	return added, health
}

// TestEndToEndConstraintFamilyDOF drives each positioning + motion + custom constraint type
// over MCP and asserts the free component's DOF — the runtime contract for the whole family.
func TestEndToEndConstraintFamilyDOF(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		kind    string
		args    func(g, f uint64, k boxKeys) map[string]any
		wantDOF int
	}{
		{"mate", "add_mate_constraint", "mate", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.topZ), "b": geomRef(f, k.topZ), "solution": "opposed"}
		}, 3},
		{"flush", "add_flush_constraint", "flush", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.topZ), "b": geomRef(f, k.topZ)}
		}, 3},
		{"angle", "add_angle_constraint", "angle", func(g, f uint64, k boxKeys) map[string]any {
			// +Z normal vs +X normal (90° apart, a non-singular start) driven to 45°.
			return map[string]any{"a": geomRef(g, k.topZ), "b": geomRef(f, k.posX), "angle": math.Pi / 4}
		}, 5},
		{"insert", "add_insert_constraint", "insert", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.edge), "b": geomRef(f, k.edge)}
		}, 1},
		{"symmetry", "add_symmetry_constraint", "symmetry", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.posX), "b": geomRef(f, k.posX), "plane": geomRef(g, k.topZ)}
		}, 3},
		{"transitional", "add_transitional_constraint", "transitional", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(f, k.topZ), "b": geomRef(g, k.topZ)}
		}, 5},
		{"rotate-rotate", "add_rotate_rotate_constraint", "rotate-rotate", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.edge), "b": geomRef(f, k.edge), "ratio": 2.0}
		}, 6},
		{"rotate-translate", "add_rotate_translate_constraint", "rotate-translate", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.edge), "b": geomRef(f, k.edge), "distance": 6.28}
		}, 6},
		{"translate-translate", "add_translate_translate_constraint", "translate-translate", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.edge), "b": geomRef(f, k.edge), "ratio": 0.5}
		}, 6},
		{"custom", "add_custom_constraint", "custom", func(g, f uint64, k boxKeys) map[string]any {
			return map[string]any{"a": geomRef(g, k.topZ), "b": geomRef(f, k.topZ), "kind": "weld", "params": []float64{1}}
		}, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs, g, f, k := twoBoxes(t)
			added, health := addConstraint(t, cs, tc.tool, tc.args(g, f, k), f)
			if added.Constraint.Type != tc.kind {
				t.Fatalf("added type = %q, want %q", added.Constraint.Type, tc.kind)
			}
			if got := dofOf(health, f); got != tc.wantDOF {
				t.Errorf("free component DOF after %s = %d, want %d (report %+v)", tc.name, got, tc.wantDOF, health)
			}
			var list wire.ConstraintsResult
			callJSON(t, cs, "list_assembly_constraints", nil, &list)
			if len(list.Constraints) != 1 {
				t.Errorf("list after %s = %d constraints, want 1", tc.name, len(list.Constraints))
			}
		})
	}
}

// TestEndToEndTangentConstraint drives the tangent constraint, which needs a real cylindrical
// face: a grounded box's vertical (+X) face and a free part's drilled bore (an analytic
// cylinder, axis +Z) — already perpendicular, so the solve holds the axis one radius from the
// plane, removing two DOF (free component left with four).
func TestEndToEndTangentConstraint(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	// Grounded plain box.
	var plain wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "plain.obk"}, &plain)
	plainKeys := readBoxKeys(t, cs)

	// Free box with a drilled bore → an analytic cylinder face.
	var bored wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "bored.obk"}, &bored)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"},
	}, nil)
	if healthy, reason := applyFeature(t, cs, "hole", map[string]any{"faceRef": topFaceKey(t, cs), "diameter": "8 mm", "depth": "20 mm"}); !healthy {
		t.Fatalf("bore unhealthy: %s", reason)
	}
	boreKey := cylinderFaceKey(t, cs)
	if boreKey == "" {
		t.Fatal("no cylindrical bore face found")
	}

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var g, f wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{"document": plain.ID, "name": "plain:1", "transform": identityCells}, &g)
	callJSON(t, cs, "place_component", map[string]any{"document": bored.ID, "name": "bored:1", "transform": identityCells}, &f)
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": g.Occurrence.ID, "grounded": true}, nil)

	added, health := addConstraint(t, cs, "add_tangent_constraint", map[string]any{
		"a": geomRef(g.Occurrence.ID, plainKeys.posX), "b": geomRef(f.Occurrence.ID, boreKey),
	}, f.Occurrence.ID)
	if added.Constraint.Type != "tangent" {
		t.Fatalf("added type = %q, want tangent", added.Constraint.Type)
	}
	if got := dofOf(health, f.Occurrence.ID); got != 4 {
		t.Errorf("tangent free DOF = %d, want 4 (report %+v)", got, health)
	}
}
