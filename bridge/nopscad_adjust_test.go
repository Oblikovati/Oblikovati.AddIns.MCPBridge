// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopAdjust(t *testing.T) {
	cs := freshPart(t)
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "2.77 mm", "2.77 mm")
	applyNew(t, cs, s, 0, "1.78 mm", "adjust dial")
	addBoxFeature(t, cs, [][]float64{{-0.16, -0.032}, {0.16, 0.032}}, "3.2 mm", "0.64 mm", "1.1 mm", "cut", "adjust slot x")
	addBoxFeature(t, cs, [][]float64{{-0.032, -0.16}, {0.032, 0.16}}, "0.64 mm", "3.2 mm", "1.1 mm", "cut", "adjust slot y")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("adjust volume = %.6f, want slotted dial", got)
	}
}
