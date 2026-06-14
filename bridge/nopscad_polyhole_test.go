// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopPolyhole(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "polyR", "2.5 mm")
	addNopParam(t, cs, "polyH", "12 mm")
	s := addSketchOn(t, cs)
	points := regularPolygon(8, 0.25, math.Pi/8)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "polyline", "closed": true, "points": points}, nil)
	assertExtrudeVolume(t, cs, s, 0, "polyH", polygonArea(points)*1.2, "polyhole")
}
