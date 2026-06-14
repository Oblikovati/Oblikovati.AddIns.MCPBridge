// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopEllipticalCableStrip(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "stripW", "10 mm")
	s := addSketchOn(t, cs)
	points := semiEllipseFrame(1.5, 2.4, 0.08, 32)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "polyline", "closed": true, "points": points}, nil)
	assertExtrudeVolume(t, cs, s, 0, "stripW", polygonArea(points), "elliptical_cable_strip")
}
