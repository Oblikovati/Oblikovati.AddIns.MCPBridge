// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopPiCutout(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"baseW", "9 mm"}, {"stemW", "1.2 mm"}, {"baseH", "3.5 mm"}, {"gap", "8.6 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	s0 := addSketchOn(t, cs)
	pts := stadiumBandPoints2D(0.35, 0, 0.35, 0.18, 24)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "polyline", "closed": true, "points": pts}, nil)
	applyNew(t, cs, s0, 0, "baseH", "pi cutout hull")
	addBoxFeature(t, cs, [][]float64{{-0.45, -0.55}, {0.45, -0.43}}, "baseW", "stemW", "9 mm", "join", "pi cutout lower stem")
	addBoxFeature(t, cs, [][]float64{{-0.45, 0.43}, {0.45, 0.55}}, "baseW", "stemW", "9 mm", "join", "pi cutout upper stem")
	if got := partVolume(t, cs); got <= polygonArea(pts)*0.35 {
		t.Errorf("pi_cutout volume = %.6f, want larger than hulled base", got)
	}
}
