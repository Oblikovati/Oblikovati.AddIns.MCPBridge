// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopExtrusionCenterSection(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-0.1, -1}, {0.1, 1}}, "2 mm", "20 mm", "1.2 mm", "new", "extrusion vertical spar")
	addBoxFeature(t, cs, [][]float64{{-1, -0.1}, {1, 0.1}}, "20 mm", "2 mm", "1.2 mm", "join", "extrusion cross spar")
	for _, side := range []float64{-1, 1} {
		addBoxFeature(t, cs, [][]float64{{side*0.72 - 0.09, -0.55}, {side*0.72 + 0.09, 0.55}}, "1.8 mm", "11 mm", "1.2 mm", "join", "extrusion side tab")
		addBoxFeature(t, cs, [][]float64{{-0.55, side*0.72 - 0.09}, {0.55, side*0.72 + 0.09}}, "11 mm", "1.8 mm", "1.2 mm", "join", "extrusion end tab")
	}
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "2.2 mm", "2.2 mm")
	applyCut(t, cs, s, 0, "extrusion center hole")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("extrusion_center_section volume = %.6f, want positive", got)
	}
}
