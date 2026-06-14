// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopSquareButton(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"w", "12 mm"}, {"h", "3.5 mm"}, {"rivet", "1.6 mm"}, {"stem", "4 mm"}, {"cap", "6 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	addBoxFeature(t, cs, [][]float64{{-0.6, -0.6}, {0.6, 0.6}}, "w", "w", "h", "new", "button base")
	for _, x := range []float64{-0.4, 0.4} {
		for _, y := range []float64{-0.4, 0.4} {
			s := addSketchOn(t, cs)
			addConstrainedCircle(t, cs, s, []float64{x, y}, "0.8 mm", "rivet/2")
			applyJoin(t, cs, s, 0, "4 mm", "button rivet")
		}
	}
	for _, c := range []struct{ d, h string }{{"stem", "3 mm"}, {"cap", "3 mm"}} {
		s := addSketchOn(t, cs)
		addConstrainedCircle(t, cs, s, []float64{0, 0}, "3 mm", c.d+"/2")
		applyJoin(t, cs, s, 0, c.h, "button cap stack")
	}
	if got := partVolume(t, cs); got <= 1.2*1.2*0.35 {
		t.Errorf("square_button volume = %.6f, want larger than base", got)
	}
}
