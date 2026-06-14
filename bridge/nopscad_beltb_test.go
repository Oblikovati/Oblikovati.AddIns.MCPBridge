// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopBeltb(t *testing.T) {
	cs := freshPart(t)
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "8 mm", "8 mm")
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "6.8 mm", "6.8 mm")
	applyNew(t, cs, s, 0, "6 mm", "belt pulley arc")
	addBoxFeature(t, cs, [][]float64{{-0.8, -0.06}, {1.0, 0.06}}, "18 mm", "1.2 mm", "6 mm", "join", "belt straight run")
	if got := partVolume(t, cs); got <= 0.12*0.6*1.8 {
		t.Errorf("beltb volume = %.6f, want straight belt plus pulley arc", got)
	}
}
