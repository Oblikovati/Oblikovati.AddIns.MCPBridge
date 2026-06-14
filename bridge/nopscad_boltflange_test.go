// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopBoltCircleFlangeCircularPattern models a bolt-circle flange the Inventor way
// and builds its ring of fastener holes with a FEATURE-LEVEL circular pattern:
// patternCircular replicates a single hole *feature* about the axis. This complements
// the existing leadnut test, which builds its bolt circle at the *sketch* level
// (add_sketch_pattern) — here the replicated unit is a 3D feature whose cut boolean is
// re-applied at each occurrence, exercising the feature DAG + recompute, not just the
// solver.
//
// Reference: the flanged coupling/leadnut family (NopSCADlib vitamins/leadnut.scad and
// vitamins/shaft_coupling.scad): a disc of diameter flangeD and thickness flangeT, a
// central bore boreD, and `count` fastener holes of diameter holeD evenly spaced on a
// bolt circle of radius boltR. The holes are axial through a flat flange, so every
// boolean is planar/exact.
func TestNopBoltCircleFlangeCircularPattern(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{
		{"flangeD", "40 mm"}, {"flangeT", "4 mm"}, {"boreD", "10 mm"},
		{"holeD", "4 mm"}, {"boltR", "15 mm"},
	} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	const count = 6 // fasteners on the bolt circle

	// --- 1. Flange disc with a central bore (one annular profile) ---
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	outer := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	bore := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.5 cm"})
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	con("ground", o)
	con("coincident", outer[1], o)
	con("coincident", bore[1], o)
	dim("radius", "flangeD / 2", outer[0])
	dim("radius", "boreD / 2", bore[0])
	requireConstrained(t, cs, 0)

	annulus := profileWithHole(t, cs)
	if annulus < 0 {
		t.Fatal("no annular flange profile (outer ring with central bore) found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": annulus, "distance": "flangeT", "operation": "new",
	}); !healthy {
		t.Fatalf("flange extrude unhealthy: %s", reason)
	}

	// --- 2. One seed bolt hole on the bolt circle (a cut feature) ---
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	o2 := idsOf(t, cs, map[string]any{"sketchIndex": 1, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	hole := idsOf(t, cs, map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{1.5, 0}}, "radius": "0.2 cm"})
	con1 := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 1, "kind": kind, "entities": ents}, nil)
	}
	con1("ground", o2)
	con1("horizontal", o2, hole[1]) // hole center on the X axis
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 1, "kind": "distance", "entities": []uint64{o2, hole[1]}, "expression": "boltR"}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 1, "kind": "radius", "entities": []uint64{hole[0]}, "expression": "holeD / 2"}, nil)
	requireConstrained(t, cs, 1)

	seed, healthy, reason := addNamedFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all",
	})
	if !healthy {
		t.Fatalf("seed bolt-hole cut unhealthy: %s", reason)
	}

	// --- 3. THE FEATURE-LEVEL CIRCULAR PATTERN: replicate the hole about the axis ---
	if healthy, reason := applyFeature(t, cs, "patternCircular", map[string]any{
		"sourceFeatures": []string{seed}, "count": count, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	}); !healthy {
		t.Fatalf("patternCircular unhealthy: %s", reason)
	}

	// Volume: annular disc minus `count` cylindrical holes, in cm^3.
	want := func(flangeDmm float64) float64 {
		R, rb, rh, th := flangeDmm/20, 10.0/20, 4.0/20, 4.0/10
		return math.Pi * ((R*R - rb*rb) - count*rh*rh) * th
	}
	if got, w := partVolume(t, cs), want(40); math.Abs(got-w)/w > 0.02 {
		t.Errorf("flange volume = %.6f cm^3, want ~%.6f (2%% band) — the %d-hole pattern did not cut cleanly", got, w, count)
	}

	// Parametric resize: a wider flange must rebuild the disc AND keep all patterned
	// holes (the bolt circle stays inside the annulus), tracking the volume.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "flangeD", "expression": "50 mm"}, nil)
	if got, w := partVolume(t, cs), want(50); math.Abs(got-w)/w > 0.02 {
		t.Errorf("resized flange volume = %.6f cm^3, want ~%.6f (2%% band)", got, w)
	}
}
