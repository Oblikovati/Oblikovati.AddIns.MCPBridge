// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopHangingHole(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "supportW", "9 mm")
	addNopParam(t, cs, "supportH", "5 mm")
	addNopParam(t, cs, "holeR", "2.5 mm")
	addNopParam(t, cs, "holeH", "14 mm")
	s0 := addSketchOn(t, cs)
	addConstrainedCornerRectangle(t, cs, s0, [][]float64{{-0.45, -0.45}, {0.45, 0.45}}, "supportW", "supportW")
	assertExtrudeVolume(t, cs, s0, 0, "supportH", 0.9*0.9*0.5, "hanging_hole support")

	s1 := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s1, []float64{0, 0}, "2.5 mm", "holeR")
	applyJoin(t, cs, s1, 0, "holeH", "hanging_hole column")
	want := 0.9*0.9*0.5 + math.Pi*0.25*0.25*1.4 - math.Pi*0.25*0.25*0.5
	checkPartVolume(t, cs, want, 0.03, "hanging_hole")
}
