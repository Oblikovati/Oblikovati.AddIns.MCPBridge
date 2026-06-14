// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopPulleyGrooved models a flanged belt pulley (NopSCADlib vitamins/pulley.scad) the
// Inventor way: an 8-segment half-section — central bore, two flanges at the outer radius, and
// a recessed belt channel between them — revolved 360° about the Y axis. It is the grooved
// revolve (a stepped profile with an external recess) plus a through bore. The volume is the
// flanged cylinder minus the channel ring: π(R²−r_bore²)·w − π(R²−channel_r²)·cw. Widening the
// flange (a parameter edit) grows it.
//
// Reference: NopSCADlib/vitamins/pulley.scad (a smooth/flanged pulley: hub, flanges, channel,
// bore — the teeth, an OpenSCAD hull() per tooth, are a separable refinement).
func TestNopPulleyGrooved(t *testing.T) {
	cs := freshPart(t)
	pulleyProfile(t, cs)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("pulley section not fully constrained: dof=%d, want 0", solve.DOF)
	}

	prof := closedProfileIndex(t, cs)
	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": prof, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}

	if got, w := partVolume(t, cs), pulleyVolume(18, 5, 15, 6, 8); math.Abs(got-w)/w > 0.02 {
		t.Errorf("pulley volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Parametric: a bigger flange diameter grows both the body and the channel walls.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "flangeD", "expression": "22 mm"}, nil)
	if got, w := partVolume(t, cs), pulleyVolume(22, 5, 15, 6, 8); math.Abs(got-w)/w > 0.02 {
		t.Errorf("widened pulley volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// pulleyVolume = flanged cylinder minus the channel ring, cm^3 (all args mm).
func pulleyVolume(flangeDmm, boreDmm, channelDmm, cwMM, widthMM float64) float64 {
	R, rb, rc := flangeDmm/20, boreDmm/20, channelDmm/20
	w, cw := widthMM/10, cwMM/10
	return math.Pi*(R*R-rb*rb)*w - math.Pi*(R*R-rc*rc)*cw
}

// pulleyProfile builds the fully-constrained 8-segment pulley half-section on sketch 0.
func pulleyProfile(t *testing.T, cs *mcp.ClientSession) {
	t.Helper()
	callJSON(t, cs, "add_parameter", map[string]any{"name": "flangeD", "expression": "18 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "boreD", "expression": "5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "channelD", "expression": "15 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ft", "expression": "1 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "cw", "expression": "6 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "width", "expression": "8 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Half-section (cm seeds). r_bore=.25, R=.9, ft=.1, channel_r=.75, cw=.6, w=.8.
	mkLine := func(x0, y0, x1, y1 float64) []uint64 {
		return idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l := [8][]uint64{
		mkLine(0.25, 0, 0.9, 0),      // L0 bottom
		mkLine(0.9, 0, 0.9, 0.1),     // L1 bottom flange outer
		mkLine(0.9, 0.1, 0.75, 0.1),  // L2 step in
		mkLine(0.75, 0.1, 0.75, 0.7), // L3 channel wall
		mkLine(0.75, 0.7, 0.9, 0.7),  // L4 step out
		mkLine(0.9, 0.7, 0.9, 0.8),   // L5 top flange outer
		mkLine(0.9, 0.8, 0.25, 0.8),  // L6 top
		mkLine(0.25, 0.8, 0.25, 0),   // L7 bore wall
	}
	a := func(i int) uint64 { return l[i][1] }
	b := func(i int) uint64 { return l[i][2] }
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	for i := 0; i < 8; i++ {
		con("coincident", b(i), a((i+1)%8)) // chain into a closed loop
	}
	for _, i := range []int{0, 2, 4, 6} {
		con("horizontal", a(i), b(i))
	}
	for _, i := range []int{1, 3, 5, 7} {
		con("vertical", a(i), b(i))
	}
	con("ground", o)
	con("horizontal", o, a(0)) // bottom edge on z = 0

	dim := func(expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": ents, "expression": expr}, nil)
	}
	dim("boreD/2", o, a(0))                   // bore radius
	dim("flangeD/2", o, b(0))                 // flange radius (P1 on z=0)
	dim("ft", a(1), b(1))                     // bottom flange thickness
	dim("(flangeD - channelD)/2", a(2), b(2)) // step in → channel radius
	dim("cw", a(3), b(3))                     // channel width
	dim("(flangeD - channelD)/2", a(4), b(4)) // step out
	dim("width", a(7), b(7))                  // total width (bore wall height)
}
