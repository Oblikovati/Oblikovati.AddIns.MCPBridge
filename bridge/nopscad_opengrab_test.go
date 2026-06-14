// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopOpengrabTarget models NopSCADlib's OpenGrab target plate: a 40 mm square
// silicon-steel sheet with four corner screw holes and two side holes. It exercises
// repeated cut profiles from one sketch and checks that the resulting perforated
// plate volume tracks the OpenSCAD difference of one square and six circles.
//
// Reference: NopSCADlib/vitamins/opengrab.scad
func TestNopOpengrabTarget(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "targetSide", "40 mm")
	addNopParam(t, cs, "targetT", "1 mm")
	addNopParam(t, cs, "cornerHoleR", "1.6 mm")
	addNopParam(t, cs, "sideHoleR", "2 mm")
	s0 := addSketchOn(t, cs)
	addConstrainedCornerRectangle(t, cs, s0, [][]float64{{-2, -2}, {2, 2}}, "targetSide", "targetSide")
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s0, "profileIndex": 0, "distance": "targetT", "operation": "new",
	}); !healthy {
		t.Fatalf("target plate extrude unhealthy: %s", reason)
	}

	s1 := addSketchOn(t, cs)
	for _, c := range [][2]float64{{-1.69, -1.69}, {-1.69, 1.69}, {1.69, -1.69}, {1.69, 1.69}} {
		addConstrainedCircle(t, cs, s1, []float64{c[0], c[1]}, "1.6 mm", "cornerHoleR")
	}
	for _, c := range [][2]float64{{-1.65, 0}, {1.65, 0}} {
		addConstrainedCircle(t, cs, s1, []float64{c[0], c[1]}, "2 mm", "sideHoleR")
	}
	for pi := 0; pi < 6; pi++ {
		if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
			"sketchIndex": s1, "profileIndex": pi, "operation": "cut", "extent": "through-all",
		}); !healthy {
			t.Fatalf("target hole cut %d unhealthy: %s", pi, reason)
		}
	}

	want := 4.0*4.0*0.1 - 4*math.Pi*0.16*0.16*0.1 - 2*math.Pi*0.2*0.2*0.1
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.02 {
		t.Errorf("opengrab target volume = %.6f cm^3, want ~%.6f", got, want)
	}
}
