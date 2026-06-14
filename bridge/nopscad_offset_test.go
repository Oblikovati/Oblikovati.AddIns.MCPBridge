// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopOffsetRegion exercises the 2D-region offset (OpenSCAD offset(r)) that NopSCADlib's
// printed_box outer/inner shells are built from (offset(±wall/2) of a rounded rectangle). A
// rectangle profile is grown by d with rounded corners; the band between the original and the
// offset is an annular profile whose area is exactly the Minkowski ring 2(w+h)d + πd². It is
// extruded and the volume checked; the extrude thickness stays parametric.
//
// Reference: NopSCADlib/printed/printed_box.scad (pbox_outer_shape = offset(wall/2) …).
func TestNopOffsetRegion(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "th", "expression": "2 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Inner rectangle 20×14 mm (2×1.4 cm), corner at origin, fully constrained.
	board := rectFull(t, cs, [][]float64{{0, 0}, {2, 1.4}})
	bl, br, tr, tl := board.points[0], board.points[1], board.points[2], board.points[3]
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", bl)
	con("horizontal", bl, br)
	con("horizontal", tl, tr)
	con("vertical", bl, tl)
	con("vertical", br, tr)
	dim := func(expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": ents, "expression": expr}, nil)
	}
	dim("20 mm", bl, br)
	dim("14 mm", bl, tl)
	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("rectangle not fully constrained: dof=%d", solve.DOF)
	}

	// Grow the rectangle region by d = 3 mm with rounded corners → an outer rounded rectangle.
	prof0 := 0
	var off struct {
		Created []uint64 `json:"created"`
	}
	callJSON(t, cs, "offset_sketch", map[string]any{
		"sketchIndex": 0, "profileIndex": prof0, "distance": "3 mm", "arcSegments": 16,
	}, &off)
	if len(off.Created) < 4 {
		t.Fatalf("region offset created %d lines, want a closed loop", len(off.Created))
	}

	// The band between the original rectangle and its offset is the annular profile.
	band := profileWithHole(t, cs)
	if band < 0 {
		t.Fatal("no annular (offset band) profile found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": band, "distance": "th", "operation": "new"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	// Ring area = 2(w+h)d + πd² (Minkowski band); volume = area·thickness, cm^3.
	wantVol := func(thMM float64) float64 {
		w, h, d := 2.0, 1.4, 0.3
		ring := 2*(w+h)*d + math.Pi*d*d
		return ring * (thMM / 10)
	}
	if got, w := partVolume(t, cs), wantVol(2); math.Abs(got-w)/w > 0.02 {
		t.Errorf("offset-band volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// The extrude thickness stays parametric.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "th", "expression": "5 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(5); math.Abs(got-w)/w > 0.02 {
		t.Errorf("thicker offset-band volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
