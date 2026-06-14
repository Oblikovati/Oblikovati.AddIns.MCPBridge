// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopTrimpot3362(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-0.3495, -0.33}, {0.3495, 0.33}}, "6.99 mm", "6.6 mm", "4.5 mm", "new", "trimpot body")
	for _, p := range [][]float64{{-0.26, -0.22}, {0.26, -0.22}, {0, 0.22}} {
		addBoxFeature(t, cs, [][]float64{{p[0] - 0.019, p[1] - 0.019}, {p[0] + 0.019, p[1] + 0.019}}, "0.38 mm", "0.38 mm", "0.38 mm", "join", "trimpot foot")
	}
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "2.77 mm", "2.77 mm")
	applyCut(t, cs, s, 0, "trimpot adjust recess")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("trimpot3362 volume = %.6f, want positive body plus feet", got)
	}
}
