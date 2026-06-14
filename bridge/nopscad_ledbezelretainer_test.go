// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopLedBezelRetainer(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"or", "4.5 mm"}, {"ir", "3.2 mm"}, {"h", "4 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "4.5 mm", "or")
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "3.2 mm", "ir")
	applyNew(t, cs, s, 0, "h", "led bezel retainer")
	want := math.Pi * (0.45*0.45 - 0.32*0.32) * 0.4
	checkPartVolume(t, cs, want, 0.03, "led_bezel_retainer")
}
