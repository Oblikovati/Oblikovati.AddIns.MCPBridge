// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopRibbonGrommetHole(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "holeH", "50 mm")
	pts := ribbonGrommetProfile2D(2.72, 0.405, 0.15, 16)
	s := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "polyline", "closed": true, "points": pts}, nil)
	assertExtrudeVolume(t, cs, s, 0, "holeH", polygonArea(pts)*5.0, "ribbon_grommet_hole")
}
