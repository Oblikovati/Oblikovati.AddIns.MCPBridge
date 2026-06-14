// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopBentTubeSweep sweeps a circle along a CURVED (90° arc) path — a bent tube / pipe
// elbow (NopSCADlib ht_pipe bends). The only sweep covered so far is a straight rail; a
// turning path is where sweep-frame bugs live: the profile must stay perpendicular to the
// path around the bend, or the tube comes out oblique/pinched. The volume is the Pappus
// value (profile area × the distance its centroid travels along the arc), independent of
// orientation IFF the profile is carried perpendicular — so a broken sweep frame shows up as
// a volume error.
func TestNopBentTubeSweep(t *testing.T) {
	cs := freshPart(t)
	const r, bend = 0.2, 2.0 // tube radius, bend (centerline) radius, cm

	// Profile: a circle on XY (normal +Z), centred at the origin (the path's start), fully
	// constrained.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	prof := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.2 cm"})
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{o}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "coincident", "entities": []uint64{o, prof[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{prof[0]}, "expression": "0.2 cm"}, nil)
	requireConstrained(t, cs, 0)

	// Path: a 90° arc on XZ (sketch Y → world Z) from the origin (tangent +Z, matching the
	// profile plane) curving over to +X. center=(bend,0), start=(0,0), end=(bend,bend), CW.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XZ"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": 1, "kind": "arc",
		"points": [][]float64{{bend, 0}, {0, 0}, {bend, bend}}, "ccw": false,
	}, nil)

	if healthy, reason := applyFeature(t, cs, "sweep", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "pathSketchIndex": 1, "pathIndex": 0,
	}); !healthy {
		t.Fatalf("bent-tube sweep unhealthy: %s", reason)
	}

	// Pappus: V = πr² · (centroid arc length) = πr² · (π/2 · bend).
	want := math.Pi * r * r * (math.Pi / 2 * bend)
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.04 {
		t.Errorf("bent-tube volume = %.6f cm^3, want ~%.6f (4%% band) — the sweep frame did not stay perpendicular around the bend", got, want)
	}
}
