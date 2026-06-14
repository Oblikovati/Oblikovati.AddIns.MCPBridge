// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// partCentroidX reads the active part's centre-of-mass x (cm).
func partCentroidX(t *testing.T, cs *mcp.ClientSession) float64 {
	t.Helper()
	var pp struct {
		Centroid [3]float64 `json:"centroid"`
	}
	callJSON(t, cs, "get_physical_properties", nil, &pp)
	return pp.Centroid[0]
}

// TestNopLoftCenterline is the integration guard for S4 (kLoftWithCenterline) over the router path
// the live app uses: a loft between two equal circles with a CENTERLINE that bows to x=2 must bend
// along it — the body's centre of mass moves off-axis (a straight loft is centred).
func TestNopLoftCenterline(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{c0[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{c0[0]}, "expression": "2 cm"}, nil)
	requireDOF(t, cs, 0)
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
	// Centerline: a polyline on XZ ((u,v)→(u,0,v)) bowing to x=2 at mid height.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XZ"}, nil)
	spineSketch := 2
	idsOf(t, cs, map[string]any{"sketchIndex": spineSketch, "kind": "polyline", "points": [][]float64{{0, 0}, {2, 2}, {0, 4}}})

	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections":   []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
		"centerline": map[string]any{"pathSketchIndex": spineSketch, "pathIndex": 0},
	}); !healthy {
		t.Fatalf("centerlined loft unhealthy: %s", reason)
	}
	if cx := partCentroidX(t, cs); cx < 0.5 {
		t.Fatalf("centerline did not bend the loft: centroid x = %.3f cm, want > 0.5 (straight ≈0)", cx)
	}
}
