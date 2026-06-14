// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// topPlanarFaceRef returns the reference key of the highest planar face of the active part's first
// body (its top cap), selected by surface kind + bbox-centre height from get_reference_keys.
func topPlanarFaceRef(t *testing.T, cs *mcp.ClientSession) string {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Kind  string    `json:"kind"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		t.Fatal("get_reference_keys returned no bodies")
	}
	key, bestZ := "", -1e30
	for _, f := range rk.Bodies[0].Faces {
		if f.Kind == "plane" && len(f.Point) == 3 && f.Point[2] > bestZ {
			key, bestZ = f.Key, f.Point[2]
		}
	}
	if key == "" {
		t.Fatal("no planar face found in get_reference_keys")
	}
	return key
}

// TestNopLoftFaceTangent is the integration guard for S2c over the router path the live app uses:
// a loft from a cylinder's top FACE (a face section) up to a smaller circle, with a Tangent
// condition, must flare out tangent to the planar top — holding measurably more volume than the
// ruled (Free) frustum between the same boundaries.
func TestNopLoftFaceTangent(t *testing.T) {
	loftFromCyl := func(condition string) float64 {
		cs := freshPart(t)
		// Cylinder: extrude a circle r=2 up 3 mm.
		callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
		c0 := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{c0[1]}}, nil)
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{c0[0]}, "expression": "2 cm"}, nil)
		requireDOF(t, cs, 0)
		if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "30 mm"}); !healthy {
			t.Fatalf("cylinder extrude unhealthy: %s", reason)
		}
		faceRef := topPlanarFaceRef(t, cs)

		// Small circle 30 mm above the top cap, on a work plane.
		var wp struct {
			Index int `json:"index"`
		}
		callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "60 mm"}, &wp)
		var sk struct {
			SketchIndex int `json:"sketchIndex"`
		}
		callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
		c1 := idsOf(t, cs, map[string]any{"sketchIndex": sk.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "1 cm"})
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "radius", "entities": []uint64{c1[0]}, "expression": "1 cm"}, nil)
		requireDOF(t, cs, sk.SketchIndex)

		args := map[string]any{
			"sections":  []map[string]any{{"faceRef": faceRef}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
			"operation": "new",
		}
		if condition != "" {
			args["first"] = map[string]any{"condition": condition}
		}
		if healthy, reason := applyFeature(t, cs, "loft", args); !healthy {
			t.Fatalf("face loft (%s) unhealthy: %s", condition, reason)
		}
		return partVolume(t, cs) // the loft is a new body; total = cylinder + loft, but we compare like-for-like
	}

	free := loftFromCyl("")
	tangent := loftFromCyl("tangent")
	if tangent <= free*1.05 {
		t.Errorf("Tangent face loft did not flare: tangent total vol %.3f, free total vol %.3f (want tangent clearly larger)", tangent, free)
	}
}
