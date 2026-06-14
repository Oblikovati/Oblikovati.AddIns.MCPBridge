// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopCounterboreMount models a counterbored cap-screw mount — the screw recess pervasive in
// NopSCADlib printed parts (a wide flat recess for the cap head over a narrow clearance bore).
// It is the corpus's first end-to-end test of the COUNTERBORE hole path (the hole feature's
// type=counterbore → brep.CutCounterboreHole): one drill makes a stepped hole, a recess of
// counterDiameter×counterDepth over a bore of diameter for the rest of the depth.
//
// Reference: NopSCADlib cap-screw recesses (e.g. counterbored fixing holes in printed parts).
func TestNopCounterboreMount(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"L", "40 mm"}, {"W", "30 mm"}, {"t", "12 mm"},
		{"boreDia", "6 mm"}, {"boreDepth", "9 mm"}, {"cDia", "11 mm"}, {"cDepth", "4 mm"}} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}

	// Plate: an L×W rectangle, corner at the origin, extruded t.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	plate := rectFull(t, cs, [][]float64{{0, 0}, {4, 3}})
	bl, br, tr, tl := plate.points[0], plate.points[1], plate.points[2], plate.points[3]
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", bl)
	con("horizontal", bl, br)
	con("vertical", bl, tl)
	con("horizontal", tl, tr)
	con("vertical", br, tr)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, br}, "expression": "L"}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, tl}, "expression": "W"}, nil)
	requireConstrained(t, cs, 0)
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": 0, "profileIndex": closedProfileIndex(t, cs), "distance": "t", "operation": "new",
	}}, nil)

	// Counterbore at the top-face centroid: an 11×4 recess over a Ø6 bore, 9 deep total (blind).
	if healthy, reason := applyFeature(t, cs, "hole", map[string]any{
		"faceRef": topFaceKey(t, cs), "type": "counterbore",
		"diameter": "boreDia", "depth": "boreDepth", "counterDiameter": "cDia", "counterDepth": "cDepth",
	}); !healthy {
		t.Fatalf("counterbore unhealthy: %s", reason)
	}

	if got, w := partVolume(t, cs), counterboreVolume(4); math.Abs(got-w)/w > 0.02 {
		t.Errorf("counterbore mount volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Parametric: a deeper recess removes more (the bore-below-recess shrinks to compensate).
	callJSON(t, cs, "set_parameter", map[string]any{"name": "cDepth", "expression": "6 mm"}, nil)
	if got, w := partVolume(t, cs), counterboreVolume(6); math.Abs(got-w)/w > 0.02 {
		t.Errorf("deeper-recess volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// counterboreVolume = plate − (recess cr×cDepth + bore r×(depth−cDepth)), cm^3 (cDepth in mm;
// fixed L=40 W=30 t=12 boreØ=6 depth=9 cØ=11).
func counterboreVolume(cDepthMM float64) float64 {
	const L, W, t, r, cr, depth = 4.0, 3.0, 1.2, 0.3, 0.55, 0.9
	cd := cDepthMM / 10
	return L*W*t - (math.Pi*cr*cr*cd + math.Pi*r*r*(depth-cd))
}
