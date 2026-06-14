// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopVariacDial(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "dialR", "25 mm")
	addNopParam(t, cs, "dialT", "3 mm")
	addNopParam(t, cs, "shaftR", "5.5 mm")
	addNopParam(t, cs, "screwR", "2.5 mm")
	s0 := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s0, []float64{0, 0}, "25 mm", "dialR")
	applyNew(t, cs, s0, 0, "dialT", "variac dial")

	s1 := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s1, []float64{0, 0}, "5.5 mm", "shaftR")
	for _, p := range regularPolygon(3, 1.6, -math.Pi/2) {
		addConstrainedCircle(t, cs, s1, p, "2.5 mm", "screwR")
	}
	for pi := 0; pi < 4; pi++ {
		applyCut(t, cs, s1, pi, "variac dial hole")
	}
	want := (math.Pi*2.5*2.5 - math.Pi*0.55*0.55 - 3*math.Pi*0.25*0.25) * 0.3
	checkPartVolume(t, cs, want, 0.03, "variac_dial")
}
