// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopNutTrap models NopSCADlib's vertical nut_trap: a screw clearance cylinder
// unioned with a hexagonal nut pocket. This keeps the OpenSCAD construction shape
// rather than a manufactured part, so the important behavior is overlapping JOINs
// between a round through-hole volume and a shorter hex prism.
//
// Reference: NopSCADlib/vitamins/nut.scad
func TestNopNutTrap(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "screwR", "1.7 mm")
	addNopParam(t, cs, "nutR", "3.2 mm")
	callJSON(t, cs, "add_parameter", map[string]any{"name": "nutDepth", "expression": "2.5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "screwH", "expression": "20 mm"}, nil)

	s0 := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s0, []float64{0, 0}, "1.7 mm", "screwR")
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s0, "profileIndex": 0, "distance": "screwH", "operation": "new",
	}); !healthy {
		t.Fatalf("nut_trap screw cylinder unhealthy: %s", reason)
	}

	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": s1, "kind": "polyline", "closed": true, "points": regularPolygon(6, 0.32, math.Pi/6),
	}, nil)
	if closedProfileIndex(t, cs) < 0 {
		t.Fatal("nut_trap hex profile did not form a closed profile")
	}
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s1, "profileIndex": 0, "distance": "2 * nutDepth", "operation": "join",
	}); !healthy {
		t.Fatalf("nut_trap hex join unhealthy: %s", reason)
	}

	want := func(nutDepthMM float64) float64 {
		screwR, screwH := 0.17, 2.0
		hexR, nutH := 0.32, 2*nutDepthMM/10
		hexArea := 3 * math.Sqrt(3) * hexR * hexR / 2
		return math.Pi*screwR*screwR*screwH + (hexArea-math.Pi*screwR*screwR)*nutH
	}
	if got, w := partVolume(t, cs), want(2.5); math.Abs(got-w)/w > 0.03 {
		t.Errorf("nut_trap volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "nutDepth", "expression": "4 mm"}, nil)
	if got, w := partVolume(t, cs), want(4); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized nut_trap volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

func regularPolygon(sides int, radius float64, angleOffset float64) [][]float64 {
	points := make([][]float64, sides)
	for i := range sides {
		angle := angleOffset + 2*math.Pi*float64(i)/float64(sides)
		points[i] = []float64{radius * math.Cos(angle), radius * math.Sin(angle)}
	}
	return points
}
