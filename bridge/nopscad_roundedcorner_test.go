// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopRoundedCorner(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "cornerT", "2 mm")
	pts := roundedCornerRectPoints2D(1.6, 2.4, 0.35, 20)
	s := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "polyline", "closed": true, "points": pts}, nil)
	assertExtrudeVolume(t, cs, s, 0, "cornerT", polygonArea(pts)*0.2, "rounded_corner")
}
