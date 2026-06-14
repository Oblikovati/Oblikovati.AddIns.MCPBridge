// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestNopLoftRail is the integration guard for S3 (kLoftWithRails) over the router path the live
// app uses: a loft between two equal circles with a guide RAIL that bulges to x=3.5 must follow
// the rail, holding more volume than the un-railed (ruled) cylinder.
func TestNopLoftRail(t *testing.T) {
	cs := freshPart(t)
	// Bottom circle r=2 on XY.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{c0[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{c0[0]}, "expression": "2 cm"}, nil)
	requireDOF(t, cs, 0)
	// Top circle r=2, 40 mm up.
	var wp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "40 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	c1 := idsOf(t, cs, map[string]any{"sketchIndex": sk.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "radius", "entities": []uint64{c1[0]}, "expression": "2 cm"}, nil)
	requireDOF(t, cs, sk.SketchIndex)
	// Rail: a polyline on XZ ((u,v)→(u,0,v)) from the bottom +X corner, out to x=3.5 at mid, back.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XZ"}, nil)
	railSketch := 2
	idsOf(t, cs, map[string]any{"sketchIndex": railSketch, "kind": "polyline", "points": [][]float64{{2, 0}, {3.5, 2}, {2, 4}}})

	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
		"rails":    []map[string]any{{"pathSketchIndex": railSketch, "pathIndex": 0}},
	}); !healthy {
		t.Fatalf("railed loft unhealthy: %s", reason)
	}
	// A ruled equal-circle cylinder is π·2²·4 ≈ 50.3 cm³; the rail bulge pushes it well past that.
	if v := partVolume(t, cs); v < 53 {
		t.Fatalf("railed loft did not bulge: volume = %.3f cm³, want > 53 (ruled cylinder ≈50.3)", v)
	}
}
