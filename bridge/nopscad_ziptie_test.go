// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopZiptie(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"tieW", "3.6 mm"}, {"strapT", "1.8 mm"}, {"latchW", "3.5 mm"}, {"latchH", "3.2 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	outer := stadiumBandPoints2D(0, 0, 1.0, 0.45, 24)
	inner := stadiumBandPoints2D(0, 0, 0.82, 0.27, 24)
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "polyline", "closed": true, "points": outer}, nil)
	applyNew(t, cs, s0, 0, "tieW", "ziptie outer")
	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "polyline", "closed": true, "points": inner}, nil)
	applyCut(t, cs, s1, 0, "ziptie inner offset")
	strapVolume := partVolume(t, cs)
	addBoxFeature(t, cs, [][]float64{{0.65, -0.16}, {1.0, 0.16}}, "latchW", "latchH", "tieW", "join", "ziptie latch")
	if got := partVolume(t, cs); got <= strapVolume {
		t.Errorf("ziptie volume = %.6f, want larger than strap %.6f", got, strapVolume)
	}
}
