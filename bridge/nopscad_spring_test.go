// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopSpringCoil models a NopSCADlib compression spring: a round wire coiled
// helically about the part axis. The wire cross-section (a small circle of radius
// `wire`) sits at mean radius R on a plane containing the Z axis, fully constrained
// (centre on the axis-plane at distance R, radius dimensioned), then the coil feature
// sweeps it `revs` turns at `pitch`. Exercises the coil (helical sweep) feature with
// parametric pitch/revolutions.
//
// Reference: NopSCADlib/vitamins/spring.scad (a helical swept wire).
func TestNopSpringCoil(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "R", "expression": "5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "wire", "expression": "1 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "pitch", "expression": "3 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "revs", "expression": "5"}, nil)
	// XZ plane contains the Z coil axis; sketch X→world X, sketch Y→world Z.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XZ"}, nil)

	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	circle := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0.5, 0}}, "radius": "0.1 cm"})
	circleE, center := circle[0], circle[1]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", o)
	con("horizontal", o, center) // wire centre on the axis plane (world Z = 0)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "distance", "entities": []uint64{o, center}, "expression": "R",
	}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "radius", "entities": []uint64{circleE}, "expression": "wire",
	}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("spring sketch not fully constrained: dof=%d, want 0", solve.DOF)
	}

	if healthy, reason := applyFeature(t, cs, "coil", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/z",
		"pitch": "pitch", "revolutions": "revs",
	}); !healthy {
		t.Fatalf("coil unhealthy: %s", reason)
	}

	// Wire volume = pi*r^2 * helix length; L = revs * sqrt((2*pi*R)^2 + pitch^2). cm.
	want := func(Rmm, wireMM, pitchMM, revs float64) float64 {
		R, r, p := Rmm/10, wireMM/10, pitchMM/10
		circumference := 2 * math.Pi * R
		l := revs * math.Sqrt(circumference*circumference+p*p)
		return math.Pi * r * r * l
	}
	if got, w := partVolume(t, cs), want(5, 1, 3, 5); math.Abs(got-w)/w > 0.06 {
		t.Errorf("spring volume = %.6f cm^3, want ~%.6f (helix wire)", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "revs", "expression": "8"}, nil)
	if got, w := partVolume(t, cs), want(5, 1, 3, 8); math.Abs(got-w)/w > 0.06 {
		t.Errorf("resized spring volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
