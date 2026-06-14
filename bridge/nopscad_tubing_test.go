// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopTubingSweep models NopSCADlib tubing: a circular wire/tube section swept
// along a path. It uses two fully-constrained sketches — a circle profile on XY and a
// straight rail along +Z on XZ — and the sweep feature to build a length of tube.
// Exercises the sweep (profile-along-path) feature; volume = pi*r^2 * length.
//
// Reference: NopSCADlib/vitamins/tubing.scad (a swept circular section).
func TestNopTubingSweep(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r", "expression": "2 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "len", "expression": "20 mm"}, nil)

	// Profile: a circle on XY (normal +Z), centred at the origin.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	prof := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.2 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{prof[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{prof[0]}, "expression": "r"}, nil)
	requireDOF(t, cs, 0)

	// Rail: a straight open line up +Z, on XZ (sketch Y → world Z).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XZ"}, nil)
	path := idsOf(t, cs, map[string]any{"sketchIndex": 1, "kind": "line", "points": [][]float64{{0, 0}, {0, 2}}})
	lineP0, lineP1 := path[1], path[2]
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 1, "kind": "ground", "entities": []uint64{lineP0}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 1, "kind": "vertical", "entities": []uint64{lineP0, lineP1}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 1, "kind": "distance", "entities": []uint64{lineP0, lineP1}, "expression": "len"}, nil)
	requireDOF(t, cs, 1)

	if healthy, reason := applyFeature(t, cs, "sweep", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "pathSketchIndex": 1, "pathIndex": 0,
	}); !healthy {
		t.Fatalf("sweep unhealthy: %s", reason)
	}

	want := func(rMM, lenMM float64) float64 { rc, lc := rMM/10, lenMM/10; return math.Pi * rc * rc * lc }
	if got, w := partVolume(t, cs), want(2, 20); math.Abs(got-w)/w > 0.03 {
		t.Errorf("tubing volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "len", "expression": "30 mm"}, nil)
	if got, w := partVolume(t, cs), want(2, 30); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized tubing volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// requireDOF asserts the sketch at idx solves to zero degrees of freedom.
func requireDOF(t *testing.T, cs *mcp.ClientSession, idx int) {
	t.Helper()
	var s struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": idx}, &s)
	if s.DOF != 0 {
		t.Fatalf("sketch %d not fully constrained: dof=%d, want 0", idx, s.DOF)
	}
}
