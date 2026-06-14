// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopORingTorus models a NopSCADlib O-ring: a torus made by revolving a circular
// section about the part axis. The section circle is offset from the axis by the
// centreline radius R = id/2 + minor/4 and has radius minor/2. It is fully
// constrained (0 DOF) — circle centre grounded on the axis at R, radius dimensioned —
// then revolved 360°. Exercises revolving a closed circular profile (a torus) and
// checks the Pappus volume 2·pi^2·R·r^2 against the OpenSCAD golden O_ring(20,3).
//
// Reference: NopSCADlib/vitamins/o_ring.scad (O_ring(id, minor_d)).
func TestNopORingTorus(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "id", "expression": "20 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "minor", "expression": "3 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	circle := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{1.075, 0}}, "radius": "0.15 cm"})
	circleE, center := circle[0], circle[1]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", o)
	con("horizontal", o, center) // section centre on the revolve axis line (y = 0)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "distance", "entities": []uint64{o, center}, "expression": "id/2 + minor/4",
	}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "radius", "entities": []uint64{circleE}, "expression": "minor/2",
	}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("o-ring sketch not fully constrained: dof=%d", solve.DOF)
	}

	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}

	// Torus volume 2*pi^2*R*r^2 (cm), R=id/2+minor/4, r=minor/2.
	want := func(idMM, minorMM float64) float64 {
		r := (minorMM / 2) / 10
		R := (idMM/2 + minorMM/4) / 10
		return 2 * math.Pi * math.Pi * R * r * r
	}
	if got, w := partVolume(t, cs), want(20, 3); math.Abs(got-w)/w > 0.03 {
		t.Errorf("o-ring volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "minor", "expression": "4 mm"}, nil)
	if got, w := partVolume(t, cs), want(20, 4); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized o-ring volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
