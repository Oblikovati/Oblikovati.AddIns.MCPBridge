// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopRibbonGrommet(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "grommetT", "3 mm")
	outer := ribbonGrommetProfile2D(2.75, 0.42, 0.16, 16)
	inner := [][]float64{{-0.95, 0.08}, {0.95, 0.08}, {0.95, 0.24}, {-0.95, 0.24}}
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "polyline", "closed": true, "points": outer}, nil)
	applyNew(t, cs, s0, 0, "grommetT", "ribbon grommet side")
	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "polyline", "closed": true, "points": inner}, nil)
	applyCut(t, cs, s1, 0, "ribbon slot")
	checkPartVolume(t, cs, (polygonArea(outer)-polygonArea(inner))*0.3, 0.02, "ribbon_grommet")
}
