// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopGmShaft models NopSCADlib's gm_shaft_shape: a geared-motor output shaft — a
// Ø d cylinder with one flat milled at distance `flat` from the centre (a "D" shaft).
// It extrudes the full circle into a cylinder, then cuts the material beyond the flat
// plane with an extrude-cut, exercising a circular extrude followed by a boolean cut.
// Volume = (disc area − the cut-off circular segment) · length.
//
// Reference: NopSCADlib/vitamins/gear_motor.scad
//
//	module gm_shaft_shape(type) difference(circle(r), translate([-r+shaft.y,…]) square(…));
//	FIT0492_A shaft = [d=6, flat=5.5, …]; flat plane sits at x = -r + flat = 2.5.
func TestNopGmShaft(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "d", "expression": "6 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "len", "expression": "12 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	circle := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.3 cm"})
	circleE, center := circle[0], circle[1]
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{center}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{circleE}, "expression": "d / 2"}, nil)
	requireDOF(t, cs, 0)

	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "len", "operation": "new"}); !healthy {
		t.Fatalf("shaft cylinder extrude unhealthy: %s", reason)
	}

	// The flat plane sits at x = 2.5 mm = 0.25 cm. Cut everything with x > 0.25 cm by
	// extruding a rectangle that fully covers the +X cap region through the shaft.
	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "rectangle",
		"points": [][]float64{{0.25, -0.4}, {0.4, 0.4}}}, nil)
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s1, "profileIndex": 0, "distance": "len", "operation": "cut",
	}); !healthy {
		t.Fatalf("flat-cut extrude unhealthy: %s", reason)
	}

	// Sharp-corner analytic volume: (πR² − segment(R, a)) · L, a = flat plane offset.
	want := func(dMM, lMM float64) float64 {
		R := (dMM / 2) / 10 // cm
		L := lMM / 10
		a := 0.25 // flat plane x-offset in cm
		seg := R*R*math.Acos(a/R) - a*math.Sqrt(R*R-a*a)
		return (math.Pi*R*R - seg) * L
	}
	// Faceted cylinder cut by a plane: a small faceting band.
	if got, w := partVolume(t, cs), want(6, 12); math.Abs(got-w)/w > 0.03 {
		t.Errorf("gm_shaft volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "len", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), want(6, 16); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized gm_shaft volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
