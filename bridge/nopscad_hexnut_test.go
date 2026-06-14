// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopHexNutBlank models a hex nut blank: a regular hexagonal prism with a central
// through hole. It exercises the regular-polygon auto-constraints (a polygon is now a
// rigid regular n-gon: centre grounded, across-flats dimensioned via an edge offset,
// one vertex's rotation locked → 0 DOF), the polygon profile, and a hole-aware extrude.
//
// Reference: NopSCADlib/vitamins/nut.scad (the hex prism blank; AF = across flats).
func TestNopHexNutBlank(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "af", "expression": "10 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "hole", "expression": "5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "th", "expression": "5 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Hexagon: idsOf → [line0, v0..v5, center]. Seed circumradius ~0.577 cm (AF 10mm).
	poly := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "polygon",
		"points": [][]float64{{0, 0}, {0.57735, 0}}, "sides": 6})
	edge0 := poly[0]
	v0 := poly[1]
	center := poly[len(poly)-1]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", center)
	con("horizontal", center, v0) // lock rotation
	// Across-flats = 2 × apothem; the apothem is the centre→edge perpendicular distance.
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "offsetDim", "entities": []uint64{center, edge0}, "expression": "af/2",
	}, nil)

	// Central hole, concentric with the hex.
	circle := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.25 cm"})
	con("coincident", circle[1], center)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "radius", "entities": []uint64{circle[0]}, "expression": "hole/2",
	}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("hex-nut sketch not fully constrained: dof=%d, want 0", solve.DOF)
	}

	annulus := profileWithHole(t, cs)
	if annulus < 0 {
		t.Fatal("no hex-with-hole profile found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": annulus, "distance": "th"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	// (hex area − hole area) × thickness; regular hexagon area with across-flats w is
	// (sqrt3/2)·w^2.
	want := func(afMM, holeMM, thMM float64) float64 {
		af, hole, th := afMM/10, holeMM/10, thMM/10
		return (math.Sqrt(3)/2*af*af - math.Pi*(hole/2)*(hole/2)) * th
	}
	if got, w := partVolume(t, cs), want(10, 5, 5); math.Abs(got-w)/w > 0.02 {
		t.Errorf("hex-nut volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "af", "expression": "13 mm"}, nil)
	if got, w := partVolume(t, cs), want(13, 5, 5); math.Abs(got-w)/w > 0.02 {
		t.Errorf("resized hex-nut volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
