// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopLightStripClip models NopSCADlib's light_strip_clip: a rectangular clip
// body with the LED strip slot and light aperture removed, extruded to the clip
// depth. The OpenSCAD source authors this as a difference of three squares; here
// the resulting concave footprint is represented as one closed polyline so the
// bridge must form and extrude the same region.
//
// Reference: NopSCADlib/vitamins/light_strip.scad
func TestNopLightStripClip(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "wall", "1.8 mm")
	addNopParam(t, cs, "slotW", "10.2 mm")
	addNopParam(t, cs, "apertureW", "6 mm")
	addNopParam(t, cs, "clipDepth", "10 mm")
	addNopParam(t, cs, "clipSide", "3 mm")

	const wall = 0.18
	const slot = 1.02
	const aperture = 0.60
	const clipLength = slot + 2*wall
	const clipWidth = 0.30 + 2*wall
	const innerTop = clipWidth - 2*wall
	sourceProfile := [][]float64{
		{-clipLength / 2, -wall}, {clipLength / 2, -wall}, {clipLength / 2, clipWidth - wall},
		{aperture / 2, clipWidth - wall}, {aperture / 2, innerTop}, {slot / 2, innerTop},
		{slot / 2, 0}, {-slot / 2, 0}, {-slot / 2, innerTop}, {-aperture / 2, innerTop},
		{-aperture / 2, clipWidth - wall}, {-clipLength / 2, clipWidth - wall},
	}
	s0 := addSketchOn(t, cs)
	addConstrainedCornerRectangle(t, cs, s0, [][]float64{{-clipLength / 2, -wall}, {clipLength / 2, clipWidth - wall}}, "slotW + 2 * wall", "clipSide + 2 * wall")
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s0, "profileIndex": 0, "distance": "clipDepth", "operation": "new",
	}); !healthy {
		t.Fatalf("light_strip_clip outer extrude unhealthy: %s", reason)
	}

	s1 := addSketchOn(t, cs)
	addConstrainedCornerRectangle(t, cs, s1, [][]float64{{-slot / 2, 0}, {slot / 2, innerTop}}, "slotW", "clipSide")
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s1, "profileIndex": 0, "extent": "through-all", "operation": "cut",
	}); !healthy {
		t.Fatalf("light_strip_clip slot cut unhealthy: %s", reason)
	}

	s2 := addSketchOn(t, cs)
	addConstrainedCornerRectangle(t, cs, s2, [][]float64{{-aperture / 2, 0}, {aperture / 2, clipWidth - wall}}, "apertureW", "clipSide + wall")
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": s2, "profileIndex": 0, "extent": "through-all", "operation": "cut",
	}); !healthy {
		t.Fatalf("light_strip_clip aperture cut unhealthy: %s", reason)
	}

	want := func(depthMM float64) float64 {
		return polygonArea(sourceProfile) * (depthMM / 10)
	}
	if got, w := partVolume(t, cs), want(10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("light_strip_clip volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "clipDepth", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), want(16); math.Abs(got-w)/w > 0.02 {
		t.Errorf("resized light_strip_clip volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

func polygonArea(points [][]float64) float64 {
	var area float64
	for i := range points {
		j := (i + 1) % len(points)
		area += points[i][0] * points[j][1]
		area -= points[j][0] * points[i][1]
	}
	return math.Abs(area) / 2
}
