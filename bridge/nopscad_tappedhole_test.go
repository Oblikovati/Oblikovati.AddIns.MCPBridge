// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopTappedHole models a tapped (threaded) hole — the THREAD feature on the cylindrical
// wall of a drilled bore (the exact drill is what yields an analytic cylinder face; revolve /
// extrude facet their curved walls into planes). A cosmetic thread records the thread data and
// leaves the solid unchanged, so the volume stays block − bore; the test is that the thread
// applies healthily to the internal cylinder and survives a thickness change.
//
// Reference: NopSCADlib tapped holes / nut threads (vitamins/nut.scad, utils/thread.scad).
func TestNopTappedHole(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "th", "expression": "10 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "30 mm", "height": "30 mm"}, nil)
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "th", "operation": "new"}); !healthy {
		t.Fatalf("block extrude unhealthy: %s", reason)
	}

	// Drill a through hole — its wall is an analytic cylinder.
	if healthy, reason := applyFeature(t, cs, "hole",
		map[string]any{"faceRef": topFaceKey(t, cs), "diameter": "8 mm", "depth": "th"}); !healthy {
		t.Fatalf("hole unhealthy: %s", reason)
	}

	// Tap it: a cosmetic internal thread on the bore wall.
	bore := cylinderFaceKey(t, cs)
	if bore == "" {
		t.Fatal("no cylindrical bore face found")
	}
	if healthy, reason := applyFeature(t, cs, "thread",
		map[string]any{"faceRef": bore, "designation": "M8x1.25"}); !healthy {
		t.Fatalf("thread unhealthy: %s", reason)
	}

	// Cosmetic thread leaves the solid unchanged: volume = block − bore.
	wantVol := func(thMM float64) float64 {
		side, h, r := 3.0, thMM/10, 0.4
		return (side*side - math.Pi*r*r) * h
	}
	if got, w := partVolume(t, cs), wantVol(10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("tapped block volume = %.6f cm^3, want ~%.6f (thread is cosmetic)", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "th", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(16); math.Abs(got-w)/w > 0.02 {
		t.Errorf("thicker tapped block volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// cylinderFaceKey returns the reference key of the part's first cylindrical face (a drilled
// bore wall), selected by the surface kind reported in get_reference_keys.
func cylinderFaceKey(t *testing.T, cs *mcp.ClientSession) string {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key  string `json:"key"`
				Kind string `json:"kind"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) > 0 {
		for _, f := range rk.Bodies[0].Faces {
			if f.Kind == "cylinder" {
				return f.Key
			}
		}
	}
	return ""
}
