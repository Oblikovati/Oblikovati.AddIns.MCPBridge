// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopJack(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"jackW", "7 mm"}, {"jackH", "6 mm"}, {"jackL", "6 mm"}, {"boreR", "1.75 mm"}, {"tubeR", "3 mm"}, {"tubeL", "8.5 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	s0 := addSketchOn(t, cs)
	addConstrainedCornerRectangle(t, cs, s0, [][]float64{{-0.3, -0.35}, {0.3, 0.35}}, "jackH", "jackW")
	applyNew(t, cs, s0, 0, "jackL", "jack body")
	sBore := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, sBore, []float64{0, 0}, "1.75 mm", "boreR")
	applyCut(t, cs, sBore, 0, "jack body bore")
	s1 := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s1, []float64{0, 0}, "3 mm", "tubeR")
	addConstrainedCircle(t, cs, s1, []float64{0, 0}, "1.75 mm", "boreR")
	applyJoin(t, cs, s1, 0, "tubeL", "jack tube")
	want := (0.6*0.7*0.6 - math.Pi*0.175*0.175*0.6) + math.Pi*(0.3*0.3-0.175*0.175)*0.25
	checkPartVolume(t, cs, want, 0.03, "jack")
}
