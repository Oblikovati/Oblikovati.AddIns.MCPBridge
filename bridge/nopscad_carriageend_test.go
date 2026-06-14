// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopCarriageEnd(t *testing.T) {
	cs := freshPart(t)
	for _, x := range []float64{-0.9, 0.9} {
		addBoxFeature(t, cs, [][]float64{{x - 0.4, -0.05}, {x + 0.4, 0.65}}, "8 mm", "7 mm", "5 mm", ternaryOp(x < 0, "new", "join"), "carriage end")
		addBoxFeature(t, cs, [][]float64{{x - 0.225, 0}, {x + 0.225, 0.35}}, "4.5 mm", "3.5 mm", "7 mm", "cut", "carriage rail cutout")
	}
	if got := partVolume(t, cs); got <= 0 || got >= 2*0.8*0.7*0.5 {
		t.Errorf("carriage_end volume = %.6f, want cut end blocks", got)
	}
}
