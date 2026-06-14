// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopRingMagnet models NopSCADlib's ring magnet: a cylindrical magnet with an
// axial bore. The original revolves a rounded_square section about the axis (corner
// radius r=0.5 — a cosmetic edge break); a faithful sharp-corner CAD ring is modelled
// here by revolving the section rectangle [id/2,0]→[od/2,h] 360° about the Y axis.
// The section is fully constrained (0 DOF): rectilinear edges, the bottom edge on the
// axis-normal through the origin, and id/od/h dimensions. Volume = π(R²−r²)·h.
//
// Reference: NopSCADlib/vitamins/magnet.scad with MAG8x4x4p2 (od=8, id=4.2, h=4).
func TestNopRingMagnet(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "od", "expression": "8 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "id", "expression": "4.2 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ht", "expression": "4 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Section rectangle offset in +X so it sweeps a ring about the Y axis. ids follow
	// the washer template: [line0, bl, br, tr, tl].
	rect := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "rectangle",
		"points": [][]float64{{0.21, 0}, {0.4, 0.4}}})
	bl, br, tr, tl := rect[1], rect[2], rect[3], rect[4]
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("horizontal", bl, br)
	con("horizontal", tl, tr)
	con("vertical", bl, tl)
	con("vertical", br, tr)
	con("ground", o)         // pin the world origin into the sketch
	con("horizontal", o, bl) // bottom edge on the X axis (the revolve plane), BL.y = 0

	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	dim("distance", "id / 2", o, bl)         // inner radius
	dim("distance", "(od - id) / 2", bl, br) // wall (radial) thickness
	dim("distance", "ht", bl, tl)            // axial height

	requireDOF(t, cs, 0)

	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("ring-magnet revolve unhealthy: %s", reason)
	}

	want := func(odMM, idMM, hMM float64) float64 {
		R, r, h := odMM/20, idMM/20, hMM/10 // mm -> cm
		return math.Pi * (R*R - r*r) * h
	}
	if got, w := partVolume(t, cs), want(8, 4.2, 4); math.Abs(got-w)/w > 0.03 {
		t.Errorf("ring-magnet volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "ht", "expression": "6 mm"}, nil)
	if got, w := partVolume(t, cs), want(8, 4.2, 6); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized ring-magnet volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
