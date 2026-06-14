// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopStarWasher models a slotted (star) washer (NopSCADlib vitamins/washer.scad
// star_washer): an annular disc with a ring of radial slots cut around it, built with the
// CIRCULAR sketch pattern of a RECTANGLE (not just a circle) — 12 slots, so the extruded cap
// face carries the bore plus 12 slot holes (13 holes → the earcut planar triangulator at
// scale). The seed slot is fully constrained (centred on the +X axis by a midpoint pin), then
// patterned; widening the slot (a parameter edit) removes more material.
//
// (NopSCADlib's spokes reach the rim; this keeps them inside the annulus so each slot stays a
// clean inner loop, with an exact rectangular-slot area.)
//
// Reference: NopSCADlib/vitamins/washer.scad (star_washer).
func TestNopStarWasher(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{
		{"starD", "18 mm"}, {"bore", "6 mm"}, {"slotLen", "4 mm"}, {"slotW", "1.5 mm"}, {"th", "1 mm"},
	} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}

	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	con("ground", o)
	outer := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.9 cm"})
	con("coincident", outer[1], o)
	dim("radius", "starD/2", outer[0])
	boreC := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.3 cm"})
	con("coincident", boreC[1], o)
	dim("radius", "bore/2", boreC[0])

	// Seed slot: a rectangle in the annulus, its left edge centred on the +X axis at the inner
	// slot radius. lines = [bottom,right,top,left]; points = [bl,br,tr,tl].
	slot := rectFull(t, cs, [][]float64{{0.4, -0.075}, {0.8, 0.075}})
	bl, br, tr, tl := slot.points[0], slot.points[1], slot.points[2], slot.points[3]
	leftEdge := slot.lines[3]
	con("horizontal", bl, br)
	con("vertical", bl, tl)
	con("horizontal", tl, tr)
	con("vertical", br, tr)
	// Pin a point on the +X axis at the slot's inner radius, and make it the left edge's
	// midpoint → the slot is centred on the axis at a known radius.
	m := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0.4, 0}}})[0]
	con("horizontal", o, m)
	dim("distance", "(bore/2 + starD/2)/2 - slotLen/2", o, m)
	con("midpoint", m, leftEdge)
	dim("distance", "slotLen", bl, br) // radial length
	dim("distance", "slotW", bl, tl)   // tangential width

	// Twelve slots evenly around the centre.
	callJSON(t, cs, "add_sketch_pattern", map[string]any{
		"sketchIndex": 0, "kind": "circular", "entities": slot.lines,
		"count": 12, "angle": "360 deg", "center": []float64{0, 0},
	}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("star-washer sketch not fully constrained: dof=%d, want 0", solve.DOF)
	}

	prof := profileWithHole(t, cs)
	if prof < 0 {
		t.Fatal("no annular slotted profile found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": prof, "distance": "th", "operation": "new"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	// Volume = (disc − bore − 12·slot)·thickness, cm^3.
	wantVol := func(starDmm float64) float64 {
		R, rb, sl, sw, tt := starDmm/20, 0.3, 0.4, 0.15, 0.1
		return (math.Pi*R*R - math.Pi*rb*rb - 12*sl*sw) * tt
	}
	if got, w := partVolume(t, cs), wantVol(18); math.Abs(got-w)/w > 0.03 {
		t.Errorf("star-washer volume = %.6f cm^3, want ~%.6f (3%% band)", got, w)
	}
	// Widen the disc: the bolt-circle of slots tracks outward (ri grows) and the area grows.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "starD", "expression": "22 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(22); math.Abs(got-w)/w > 0.03 {
		t.Errorf("widened star-washer volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
