// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopLeadnutFlange models the flange of a flanged lead-screw nut (NopSCADlib
// vitamins/leadnut.scad): a disc with a central bore and N screw holes spaced evenly on a
// pitch circle — built with the CIRCULAR sketch pattern. It exercises the circular array
// (seed hole replicated about the centre) and the multi-hole extrude (disc cap face carrying
// bore + N holes → the earcut planar triangulator). A parameter edit (widen the flange) must
// rebuild and track the volume.
//
// Reference: NopSCADlib/vitamins/leadnut.scad (the flange: circle(flange_d) − bore − screw
// holes on hole_pitch at i·360/holes).
func TestNopLeadnutFlange(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "flangeD", "expression": "22 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "flangeT", "expression": "3.5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "bore", "expression": "8 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "holeD", "expression": "3.5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "pitch", "expression": "8 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}

	// Flange outer circle + central bore, both centred at the origin.
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	con("ground", o)
	flange := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "1.1 cm"})
	con("coincident", flange[1], o)
	dim("radius", "flangeD/2", flange[0])
	boreC := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.4 cm"})
	con("coincident", boreC[1], o)
	dim("radius", "bore/2", boreC[0])

	// Seed screw hole on the +X pitch radius (angle locked horizontal → 0 DOF for the seed).
	seed := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0.8, 0}}, "radius": "0.175 cm"})
	con("horizontal", o, seed[1])
	dim("distance", "pitch", o, seed[1])
	dim("radius", "holeD/2", seed[0])

	// Circular pattern: three screw holes evenly about the centre.
	callJSON(t, cs, "add_sketch_pattern", map[string]any{
		"sketchIndex": 0, "kind": "circular", "entities": []uint64{seed[0]},
		"count": 3, "angle": "360 deg", "center": []float64{0, 0},
	}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("leadnut flange sketch not fully constrained: dof=%d, want 0", solve.DOF)
	}

	prof := profileWithHole(t, cs)
	if prof < 0 {
		t.Fatal("no flange-with-holes profile found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": prof, "distance": "flangeT", "operation": "new"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	// Volume = (flange − bore − 3·screw) area × thickness, in cm^3.
	wantVol := func(flangeDmm, tmm, boreMM, holeMM float64) float64 {
		R, tt, rb, rh := flangeDmm/20, tmm/10, boreMM/20, holeMM/20
		return (math.Pi*R*R - math.Pi*rb*rb - 3*math.Pi*rh*rh) * tt
	}
	if got, w := partVolume(t, cs), wantVol(22, 3.5, 8, 3.5); math.Abs(got-w)/w > 0.02 {
		t.Errorf("leadnut flange volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Parametric: widen the flange; the bolt circle stays put, the disc grows.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "flangeD", "expression": "26 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(26, 3.5, 8, 3.5); math.Abs(got-w)/w > 0.02 {
		t.Errorf("widened leadnut flange volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
