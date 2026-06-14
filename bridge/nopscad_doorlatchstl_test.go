// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopDoorLatchStl(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"length", "35 mm"}, {"width", "12 mm"}, {"th", "5 mm"}, {"bossH", "14.25 mm"}, {"screwR", "2.2 mm"}, {"nutR", "4.2 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	addBoxFeature(t, cs, [][]float64{{-1.75, -0.6}, {1.75, 0.6}}, "length", "width", "th", "new", "door latch base")
	addBoxFeature(t, cs, [][]float64{{-1.75, -0.2}, {1.75, 0.2}}, "length", "4 mm", "8.5 mm", "join", "door latch ridge")
	sBoss := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, sBoss, []float64{0, 0}, "6 mm", "width/2")
	applyJoin(t, cs, sBoss, 0, "bossH", "door latch boss")
	sScrew := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, sScrew, []float64{0, 0}, "2.2 mm", "screwR")
	applyCut(t, cs, sScrew, 0, "door latch screw")
	sNut := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sNut, "kind": "polyline", "closed": true, "points": regularPolygon(6, 0.42, math.Pi/6)}, nil)
	applyCut(t, cs, sNut, 0, "door latch nut trap")
	if got := partVolume(t, cs); got <= 3.5*1.2*0.5 {
		t.Errorf("door_latch_stl volume = %.6f, want larger than plate", got)
	}
}
