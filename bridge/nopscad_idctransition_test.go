// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopIDCTransition(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "pitch", "2.54 mm")
	addBoxFeature(t, cs, [][]float64{{-0.889, 0}, {0.889, 0.74}}, "17.78 mm", "7.4 mm", "6 mm", "new", "idc base")
	for i := 0; i < 10; i++ {
		x := 0.127 * (float64(i) - 4.5)
		s := addSketchOn(t, cs)
		addConstrainedCircle(t, cs, s, []float64{x, 0.37}, "0.64 mm", "pitch/4")
		applyCut(t, cs, s, 0, "idc pin hole")
	}
	addBoxFeature(t, cs, [][]float64{{-0.635, 0.285}, {0.635, 0.37}}, "12.7 mm", "0.8466666667 mm", "8 mm", "cut", "idc slot")
	for x := 0; x < 5; x++ {
		for y := 0; y < 2; y++ {
			cx := 0.254 * (float64(x) - 2)
			cy := 0.254 * (float64(y) - 0.5)
			addBoxFeature(t, cs, [][]float64{{cx - 0.025, cy - 0.025}, {cx + 0.025, cy + 0.025}}, "0.5 mm", "0.5 mm", "5 mm", "join", "idc pin")
		}
	}
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("idc_transition volume = %.6f, want positive", got)
	}
}
