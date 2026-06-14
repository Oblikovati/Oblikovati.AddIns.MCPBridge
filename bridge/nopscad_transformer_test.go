// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopTransformer(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-2, -1.5}, {2, 1.5}}, "40 mm", "30 mm", "2 mm", "new", "transformer foot")
	for _, x := range []float64{-1.5, 1.5} {
		for _, y := range []float64{-1, 1} {
			s := addSketchOn(t, cs)
			addConstrainedCircle(t, cs, s, []float64{x, y}, "1.6 mm", "1.6 mm")
			applyCut(t, cs, s, 0, "transformer hole")
		}
	}
	addBoxFeature(t, cs, [][]float64{{-1.6, -0.6}, {1.6, 0.6}}, "32 mm", "12 mm", "18 mm", "join", "transformer laminations")
	addBoxFeature(t, cs, [][]float64{{-1, -1.1}, {1, 1.1}}, "20 mm", "22 mm", "11 mm", "join", "transformer bobbin")
	if got := partVolume(t, cs); got <= 4.0*3.0*0.2 {
		t.Errorf("transformer volume = %.6f, want larger than foot", got)
	}
}
