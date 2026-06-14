// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopOffsetConcaveL exercises the 2D-region offset on a CONCAVE outline — an L-shape with
// one reflex (inner) corner — authored with the new `polyline` entity. The convex offset
// (rectangle) was already covered; the reflex corner is where offset bugs hide (the inner
// corner rounds with a concave arc; the edge bands overlap there). For the outward Minkowski
// offset of any simple polygon the band area is exactly P·d + π·d² (Steiner), independent of
// concavity — so a mishandled reflex corner shows up as a band-area error.
//
// Doubles as the end-to-end check that `polyline` builds an arbitrary closed profile over the API.
func TestNopOffsetConcaveL(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "th", "expression": "2 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// L outline (cm): 3×3 square with a 1.5×1.5 notch; one reflex corner at (1.5,1.5).
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "polyline", "closed": true,
		"points": [][]float64{{0, 0}, {3, 0}, {3, 1.5}, {1.5, 1.5}, {1.5, 3}, {0, 3}},
	}, nil)
	if idx := closedProfileIndex(t, cs); idx < 0 {
		t.Fatal("polyline L did not form a closed profile")
	}

	// Grow the L region outward by d = 3 mm (rounded corners) → an outer L.
	var off struct {
		Created []uint64 `json:"created"`
	}
	callJSON(t, cs, "offset_sketch", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "3 mm", "arcSegments": 16,
	}, &off)
	if len(off.Created) < 6 {
		t.Fatalf("L offset created %d entities, want a closed offset loop", len(off.Created))
	}

	band := profileWithHole(t, cs)
	if band < 0 {
		t.Fatal("no annular (offset band) profile around the L")
	}
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": band, "distance": "th", "operation": "new"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	// Steiner band area for the outward Minkowski offset: P·d + π·d² (cm²); volume = area·th.
	wantVol := func(thMM float64) float64 {
		const perim, d = 12.0, 0.3 // L perimeter 3+1.5+1.5+1.5+1.5+3 cm; d=3 mm
		return (perim*d + math.Pi*d*d) * (thMM / 10)
	}
	if got, w := partVolume(t, cs), wantVol(2); math.Abs(got-w)/w > 0.03 {
		t.Errorf("concave-L offset-band volume = %.6f cm^3, want ~%.6f (3%% band) — reflex-corner offset is wrong", got, w)
	}
}
