// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopSemiTeardrop models NopSCADlib's semi_teardrop: the positive-Y half of
// the teardrop profile, extruded along Z. With the default truncation, the clipped
// positive-Y domain is a semicircular cap bounded by its diameter; the chamfer path
// is a separate refinement. The sketch is fully constrained, then rebuilt through a
// radius edit to exercise the solver/profile/extrude recompute chain.
//
// Reference: NopSCADlib/utils/core/teardrops.scad
//
//	module semi_teardrop(h, r) intersection() { teardrop(r, h=0); y >= 0; }
func TestNopSemiTeardrop(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r", "expression": "4 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "20 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	origin := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	diameter := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{-0.4, 0}, {0.4, 0}}})
	diameterE, left, right := diameter[0], diameter[1], diameter[2]
	arc := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "arc", "ccw": true,
		"points": [][]float64{{0, 0}, {0.4, 0}, {-0.4, 0}}})
	arcCenter, arcStart, arcEnd := arc[1], arc[2], arc[3]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", origin)
	con("coincident", arcCenter, origin)
	con("coincident", arcStart, right)
	con("coincident", arcEnd, left)
	con("horizontal", left, right)
	con("midpoint", origin, diameterE)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "distance", "entities": []uint64{left, right}, "expression": "2 * r",
	}, nil)

	requireDOF(t, cs, 0)

	prof := closedProfileIndex(t, cs)
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": prof, "distance": "h", "operation": "new",
	}); !healthy {
		t.Fatalf("semi_teardrop extrude unhealthy: %s", reason)
	}

	want := func(rMM, hMM float64) float64 {
		rc, hc := rMM/10, hMM/10
		return math.Pi * rc * rc * hc / 2
	}
	if got, w := partVolume(t, cs), want(4, 20); math.Abs(got-w)/w > 0.03 {
		t.Errorf("semi_teardrop volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "r", "expression": "6 mm"}, nil)
	if got, w := partVolume(t, cs), want(6, 20); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized semi_teardrop volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
