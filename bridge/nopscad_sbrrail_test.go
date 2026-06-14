// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopSbrRail(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "railLen", "40 mm")
	s := addSketchOn(t, cs)
	points := sbrRailSection2D()
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "polyline", "closed": true, "points": points}, nil)
	assertExtrudeVolume(t, cs, s, 0, "railLen - 5 mm", polygonArea(points)*3.5, "sbr_rail")
}
