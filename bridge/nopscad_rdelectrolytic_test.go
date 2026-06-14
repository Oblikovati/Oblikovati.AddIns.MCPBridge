// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopRdElectrolytic(t *testing.T) {
	cs := freshPart(t)
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "9.6 mm", "9.6 mm")
	applyNew(t, cs, s, 0, "11.5 mm", "electrolytic can")
	for _, x := range []float64{-0.125, 0.125} {
		sLead := addSketchOn(t, cs)
		addConstrainedCircle(t, cs, sLead, []float64{x, 0}, "0.25 mm", "0.25 mm")
		applyJoin(t, cs, sLead, 0, "3.2 mm", "electrolytic lead")
	}
	addBoxFeature(t, cs, [][]float64{{0.18, -0.02}, {0.4, 0.02}}, "2.2 mm", "0.4 mm", "0.4 mm", "join", "electrolytic crimp")
	if got := partVolume(t, cs); got <= math.Pi*0.44*0.44*1.0 {
		t.Errorf("rd_electrolytic volume = %.6f, want can plus leads", got)
	}
}
