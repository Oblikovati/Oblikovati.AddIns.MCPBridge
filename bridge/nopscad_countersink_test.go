// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopScrewCountersink models a countersunk (flat-head) screw (NopSCADlib screw.scad,
// hs_cs): a conical head tapering from the head radius down to the shaft radius, over a
// cylindrical shaft — revolved 360° about Y. Unlike the cap screw's rectilinear steps this
// half-section has a free DIAGONAL edge (the countersink cone), so the cone falls out of the
// dimensioned endpoints. Volume = cone frustum + shaft cylinder.
//
// Reference: NopSCADlib/vitamins/screw.scad (cs_head) + screws.scad (M3_cs_screw).
func TestNopScrewCountersink(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"headD", "6 mm"}, {"shaftD", "3 mm"}, {"headH", "1.7 mm"}, {"len", "10 mm"}} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Half-section P0(0,0)→P1(.3,0)→[cone]→P2(.15,-.17)→P3(.15,-1.17)→P4(0,-1.17)→P0.
	mk := func(x0, y0, x1, y1 float64) []uint64 {
		return idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l := [5][]uint64{
		mk(0, 0, 0.3, 0),             // L0 top (flat head face)
		mk(0.3, 0, 0.15, -0.17),      // L1 cone (free diagonal)
		mk(0.15, -0.17, 0.15, -1.17), // L2 shaft side
		mk(0.15, -1.17, 0, -1.17),    // L3 bottom
		mk(0, -1.17, 0, 0),           // L4 axis
	}
	a := func(i int) uint64 { return l[i][1] }
	b := func(i int) uint64 { return l[i][2] }
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	for i := 0; i < 5; i++ {
		con("coincident", b(i), a((i+1)%5))
	}
	con("horizontal", a(0), b(0)) // top
	con("vertical", a(2), b(2))   // shaft
	con("horizontal", a(3), b(3)) // bottom
	con("vertical", a(4), b(4))   // axis
	con("ground", a(0))           // P0 at the origin

	dim := func(expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": ents, "expression": expr}, nil)
	}
	dim("headD/2", a(0), b(0))     // head radius (P0 → P1)
	dim("headH + len", a(0), b(3)) // total axis height (P0 → P4) → bottom at −(headH+len)
	dim("shaftD/2", a(3), b(3))    // shaft radius (P3 → P4, the bottom edge)
	dim("len", a(2), b(2))         // shaft length (P2 → P3) → cone bottom at −headH

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("countersink section not fully constrained: dof=%d", solve.DOF)
	}
	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": closedProfileIndex(t, cs), "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}

	if got, w := partVolume(t, cs), countersinkVolume(6, 3, 1.7, 10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("countersink volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "len", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), countersinkVolume(6, 3, 1.7, 16); math.Abs(got-w)/w > 0.02 {
		t.Errorf("longer countersink volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// countersinkVolume = cone frustum (head) + shaft cylinder, cm^3 (args mm).
func countersinkVolume(headDmm, shaftDmm, headHmm, lenMM float64) float64 {
	hr, sr, hh, l := headDmm/20, shaftDmm/20, headHmm/10, lenMM/10
	return math.Pi*hh/3*(hr*hr+hr*sr+sr*sr) + math.Pi*sr*sr*l
}
