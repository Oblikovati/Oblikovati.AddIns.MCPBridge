// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// circleBaseAndApexPlane builds a circle (r=2 cm) on XY plus an empty sketch on a work plane h mm
// above it (to host the apex point), returning the apex sketch index.
func circleBaseAndApexPlane(t *testing.T, cs *mcp.ClientSession, hMM string) int {
	t.Helper()
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{c0[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{c0[0]}, "expression": "2 cm"}, nil)
	requireDOF(t, cs, 0)
	var wp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": hMM}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	return sk.SketchIndex
}

// TestNopLoftPointCone is the integration guard for S2b point sections over the router path: a
// circle lofted to an apex (a point section) with a Sharp condition is a cone (V = πr²h/3); a
// TangentToPlane apex domes out (more volume). Both must be valid solids.
func TestNopLoftPointCone(t *testing.T) {
	cs := freshPart(t)
	apexSketch := circleBaseAndApexPlane(t, cs, "40 mm")

	sections := []map[string]any{
		{"sketchIndex": 0, "profileIndex": 0},
		{"sketchIndex": apexSketch, "point": []float64{0, 0}},
	}
	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": sections,
		"last":     map[string]any{"condition": "sharp"},
	}); !healthy {
		t.Fatalf("point-section cone unhealthy: %s", reason)
	}
	coneWant := math.Pi * 2 * 2 / 3 * 4 // πr²h/3 (cm), r=2 h=4
	cone := partVolume(t, cs)
	if math.Abs(cone-coneWant)/coneWant > 0.03 {
		t.Fatalf("cone volume = %.4f cm³, want ≈%.4f", cone, coneWant)
	}
}

// TestNopLoftPointDome checks the TangentToPlane apex domes out past the straight cone.
func TestNopLoftPointDome(t *testing.T) {
	cs := freshPart(t)
	apexSketch := circleBaseAndApexPlane(t, cs, "40 mm")
	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": []map[string]any{
			{"sketchIndex": 0, "profileIndex": 0},
			{"sketchIndex": apexSketch, "point": []float64{0, 0}},
		},
		"last": map[string]any{"condition": "tangent-to-plane", "impact": 1.5},
	}); !healthy {
		t.Fatalf("point-section dome unhealthy: %s", reason)
	}
	coneVol := math.Pi * 2 * 2 / 3 * 4
	if dome := partVolume(t, cs); dome <= coneVol*1.1 {
		t.Errorf("tangent-to-plane apex did not dome: volume %.4f cm³, want well above the cone %.4f", dome, coneVol)
	}
}
