// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopPolyRing(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"outerR", "7 mm"}, {"innerR", "3.5 mm"}, {"ringT", "1.2 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{0, 0}, "7 mm", "outerR")
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "polyline", "closed": true, "points": regularPolygon(12, 0.35, math.Pi/12)}, nil)
	prof := profileWithHole(t, cs)
	applyNew(t, cs, s, prof, "ringT", "poly ring")
	want := (math.Pi*0.7*0.7 - polygonArea(regularPolygon(12, 0.35, math.Pi/12))) * 0.12
	checkPartVolume(t, cs, want, 0.02, "poly_ring")
}
