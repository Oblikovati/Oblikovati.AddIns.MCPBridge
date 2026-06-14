// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopWasherParametric models NopSCADlib's washer the Inventor way over the MCP
// bridge: declare the part's parameters (od/id/thickness), draw the annulus
// cross-section, fully constrain it (0 DOF) with dimensions bound to those
// parameters, revolve it into a ring, then resize via a parameter edit and confirm
// the volume tracks. This is the integration half of the NopSCADlib test program —
// it exercises the parameter DAG, the constraint solver, the revolve feature, and
// the recompute chain end to end.
//
// Reference: NopSCADlib/vitamins/washer.scad (M3_washer: OD 7, ID 3.1, thickness
// 0.5 — the true CAD thickness; OpenSCAD's render shaves 0.05 for z-fighting,
// which a faithful model does not).
func TestNopWasherParametric(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "od", "expression": "7 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "id", "expression": "3.1 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "thickness", "expression": "0.5 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Annulus cross-section, offset in +X so it revolves about the Y axis into a
	// ring. Seed coords (cm) are overridden by the parametric dimensions below.
	rect := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "rectangle",
		"points": [][]float64{{0.155, 0}, {0.35, 0.05}}})
	bl, br, _, tl := rect[1], rect[2], rect[3], rect[4]
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("horizontal", bl, br)
	con("horizontal", tl, rect[3])
	con("vertical", bl, tl)
	con("vertical", br, rect[3])
	con("ground", o)         // pin the world origin into the sketch
	con("horizontal", o, bl) // bottom edge on the X axis (BL.y = 0)

	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	dim("distance", "id / 2", o, bl)         // inner radius
	dim("distance", "(od - id) / 2", bl, br) // annulus width
	dim("distance", "thickness", bl, tl)     // thickness

	// Hard gate: the profile must be fully constrained before we build on it.
	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("washer sketch is not fully constrained: dof=%d, want 0", solve.DOF)
	}

	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}

	// Volume at the seeded parameters: annulus pi*(R^2-r^2)*h in cm^3.
	wantVol := func(odMM, idMM, thMM float64) float64 {
		R, r, h := odMM/20, idMM/20, thMM/10 // mm -> cm
		return math.Pi * (R*R - r*r) * h
	}
	got := partVolume(t, cs)
	if want := wantVol(7, 3.1, 0.5); math.Abs(got-want)/want > 0.02 {
		t.Errorf("washer volume = %.6f cm^3, want ~%.6f (2%% band)", got, want)
	}

	// Parametric resize: widen the outer diameter and confirm the volume tracks the
	// parameter through the DAG -> sketch solve -> revolve rebuild.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "od", "expression": "10 mm"}, nil)
	got = partVolume(t, cs)
	if want := wantVol(10, 3.1, 0.5); math.Abs(got-want)/want > 0.02 {
		t.Errorf("resized washer volume = %.6f cm^3, want ~%.6f (2%% band)", got, want)
	}
}
