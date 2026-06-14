// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopLoftAreaGraph is the integration guard for S5 (kLoftWithAreaGraphSections): a loft between
// two equal circles with an area graph that doubles the mid cross-section's area must bulge — the
// body holds more volume than the plain (ruled) cylinder.
func TestNopLoftAreaGraph(t *testing.T) {
	cs := freshPart(t)
	topSketch := equalCirclesPart(t, cs) // two r=2 circles, 40 mm apart (from nopscad_loftcondition_test.go)
	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": topSketch, "profileIndex": 0}},
		"areaGraph": []map[string]any{{"t": 0.5, "scale": 2.0}},
	}); !healthy {
		t.Fatalf("area-graph loft unhealthy: %s", reason)
	}
	// Plain cylinder ≈ π·2²·4 = 50.3 cm³; a 2× mid area pushes it well past that.
	if v := partVolume(t, cs); v < 55 {
		t.Fatalf("area graph did not bulge: volume = %.3f cm³, want > 55 (plain cylinder ≈50.3)", v)
	}
}

// twoSquareSections builds two 4×4 squares (corners ±2) on parallel planes 40 mm apart, returning
// the top sketch index.
func twoSquareSections(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2, 2}}}, nil)
	var wp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "40 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2, 2}}}, nil)
	return sk.SketchIndex
}

// TestNopLoftMapCurve is the integration guard for S6 (MapPointCurves): an explicit map curve
// pairing the start square's (2,2) corner with the end square's NEXT corner (-2,2) forces a 90°
// twist in the correspondence; the linearly-blended twist pinches the mid section, so the body
// holds distinctly less than the plain (auto-aligned) prism.
func TestNopLoftMapCurve(t *testing.T) {
	plain := func(withMap bool) float64 {
		cs := freshPart(t)
		top := twoSquareSections(t, cs)
		args := map[string]any{"sections": []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": top, "profileIndex": 0}}}
		if withMap {
			args["mapCurves"] = []map[string]any{{"points": [][]float64{{2, 2, 0}, {-2, 2, 4}}}}
		}
		if healthy, reason := applyFeature(t, cs, "loft", args); !healthy {
			t.Fatalf("map-curve loft (map=%v) unhealthy: %s", withMap, reason)
		}
		return partVolume(t, cs)
	}
	auto, mapped := plain(false), plain(true)
	if mapped >= auto*0.9 {
		t.Fatalf("map curve did not change the correspondence: mapped %.3f vs auto %.3f (want mapped clearly less)", mapped, auto)
	}
}
