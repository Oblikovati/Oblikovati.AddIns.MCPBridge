// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopTtrack models NopSCADlib's ttrack rail (the extruded body, before the screw
// holes): its T-slot cross-section is a 12-vertex outline extruded along the rail
// length. The outline is authored with the polyline entity — the way to build an
// arbitrary concave section over the API — and extruded; the screw counterbores in
// the original are a separate per-pitch feature out of scope for the solid-section
// test. Volume = (polygon area) · length, the polygon area computed by the shoelace
// formula over the same vertices.
//
// Reference: NopSCADlib/vitamins/ttrack.scad with ttrack_universal_19mm
//
//	(W=19, H=9.5, O=9.5, SW=14.2, SH=3.3, T=2.4).
func TestNopTtrack(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "len", "expression": "30 mm"}, nil)
	s := addSketchOn(t, cs)

	// T-track section (mm) from ttrack.scad's polygon, in source order.
	const W, H, O, SW, SH, T = 19.0, 9.5, 9.5, 14.2, 3.3, 2.4
	mm := [][2]float64{
		{-O / 2, 0}, {-W / 2, 0}, {-W / 2, -H}, {W / 2, -H},
		{W / 2, 0}, {O / 2, 0}, {O / 2, -T}, {SW / 2, -T},
		{SW / 2, -T - SH}, {-SW / 2, -T - SH}, {-SW / 2, -T}, {-O / 2, -T},
	}
	pts := make([][]float64, len(mm)) // author in cm (mm/10)
	for i, p := range mm {
		pts[i] = []float64{p[0] / 10, p[1] / 10}
	}
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": s, "kind": "polyline", "closed": true, "points": pts,
	}, nil)
	if closedProfileIndex(t, cs) < 0 {
		t.Fatal("ttrack polyline did not form a closed profile")
	}

	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s, "profileIndex": 0, "distance": "len", "operation": "new",
	}); !healthy {
		t.Fatalf("ttrack extrude unhealthy: %s", reason)
	}

	// Shoelace area of the section (cm²) × length (cm).
	area := func() float64 {
		var a float64
		for i := range mm {
			j := (i + 1) % len(mm)
			a += (mm[i][0] / 10) * (mm[j][1] / 10)
			a -= (mm[j][0] / 10) * (mm[i][1] / 10)
		}
		return math.Abs(a) / 2
	}()
	want := func(lMM float64) float64 { return area * (lMM / 10) }
	if got, w := partVolume(t, cs), want(30); math.Abs(got-w)/w > 0.02 {
		t.Errorf("ttrack volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "len", "expression": "50 mm"}, nil)
	if got, w := partVolume(t, cs), want(50); math.Abs(got-w)/w > 0.02 {
		t.Errorf("resized ttrack volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
