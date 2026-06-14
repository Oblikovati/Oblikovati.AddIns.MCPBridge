// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// partArea reads the active part's total surface area (cm²) — a curved (conditioned) loft has
// more wall area than the ruled blend, so it discriminates the takeoff that volume cannot
// (a symmetric angle takeoff barely changes volume).
func partArea(t *testing.T, cs *mcp.ClientSession) float64 {
	t.Helper()
	var pp struct {
		Area float64 `json:"area"`
	}
	callJSON(t, cs, "get_physical_properties", nil, &pp)
	return pp.Area
}

// equalCirclesPart builds a part with two equal-radius circles (r=2 cm) on parallel planes 4 cm
// apart, returning the sketch index of the top circle.
func equalCirclesPart(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r", "expression": "20 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{c0[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{c0[0]}, "expression": "r"}, nil)
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
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "radius", "entities": []uint64{c1[0]}, "expression": "r"}, nil)
	requireDOF(t, cs, sk.SketchIndex)
	return sk.SketchIndex
}

// TestNopLoftAngleCondition is the integration guard for the S2 end conditions over the router
// path the live app uses: an equal-radius two-circle loft is a straight cylinder when Free, but
// an Angle takeoff at both ends curves the walls — measurably increasing the surface area. Also
// checks the takeoff angle is parameter-driven (a steeper angle curves more, so more area).
func TestNopLoftAngleCondition(t *testing.T) {
	cs := freshPart(t)
	topSketch := equalCirclesPart(t, cs)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "takeoff", "expression": "30 deg"}, nil)

	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": topSketch, "profileIndex": 0}},
		"first":    map[string]any{"condition": "angle", "angle": "takeoff", "impact": 2},
		"last":     map[string]any{"condition": "angle", "angle": "takeoff", "impact": 2},
	}); !healthy {
		t.Fatalf("angled loft unhealthy: %s", reason)
	}

	// A ruled equal-circle cylinder is ~75 cm² (2πrh + 2πr² = 50.3 + 25.1); the curved walls
	// push it well past that.
	curved := partArea(t, cs)
	if curved < 80 {
		t.Fatalf("angled loft did not curve: area = %.3f cm², want > 80 (ruled cylinder ~75)", curved)
	}

	// Steeper takeoff (more radial) curves the walls more → even more area: the angle is
	// parameter-driven through the live recompute.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "takeoff", "expression": "15 deg"}, nil)
	if steeper := partArea(t, cs); steeper <= curved {
		t.Errorf("steeper takeoff did not curve more: 15° area %.3f <= 30° area %.3f", steeper, curved)
	}
}
