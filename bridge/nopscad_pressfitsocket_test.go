// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopPressFitSocket models NopSCADlib's press_fit_socket: a square peg socket
// that is simply a w×w×(2·h) box (the bridge_droop term is a print artifact the CAD
// model ignores). It declares the w/h parameters, draws a square, fully constrains
// it (0 DOF) with a grounded corner, rectilinear edges, and a side dimension,
// extrudes the full height, and checks the box volume w²·H. Then it resizes via a
// parameter edit and reconfirms — exercising the parameter DAG, solver, and extrude.
//
// Reference: NopSCADlib/printed/press_fit.scad
//
//	module press_fit_socket(w=5, h=50) cube([w, w, 2*h], center=true);
func TestNopPressFitSocket(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "w", "expression": "5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "10 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Corner rectangle from the origin; seed coords (cm) are overridden by the
	// parametric dimensions below. Member ids follow the washer template:
	// [line0, bl, br, tr, tl].
	rect := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "rectangle",
		"points": [][]float64{{0, 0}, {0.5, 0.5}}})
	bl, br, tr, tl := rect[1], rect[2], rect[3], rect[4]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("horizontal", bl, br)
	con("horizontal", tl, tr)
	con("vertical", bl, tl)
	con("vertical", br, tr)
	con("ground", bl) // pin one corner at the world origin

	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, br}, "expression": "w",
	}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, tl}, "expression": "w",
	}, nil)

	requireDOF(t, cs, 0)

	// Full height H = 2*h (the box depth; position is irrelevant to volume).
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "2 * h", "operation": "new",
	}); !healthy {
		t.Fatalf("socket extrude unhealthy: %s", reason)
	}

	want := func(wMM, hMM float64) float64 {
		s, H := wMM/10, (2*hMM)/10 // mm -> cm
		return s * s * H
	}
	if got, w := partVolume(t, cs), want(5, 10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("press_fit_socket volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "w", "expression": "8 mm"}, nil)
	if got, w := partVolume(t, cs), want(8, 10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("resized press_fit_socket volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
