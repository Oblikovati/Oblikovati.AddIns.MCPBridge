// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopLoftFrustum models a conical funnel/frustum: two circles of different radii
// on parallel planes, lofted. It exercises the loft feature, an offset work plane, and
// a sketch on that work plane — all fully constrained and parameter-driven (radii and
// the now-parametric work-plane offset). Volume = (pi*h/3)(R1^2 + R1*R2 + R2^2).
//
// Reference: NopSCADlib transitions/funnels (a lofted truncated cone).
func TestNopLoftFrustum(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r1", "expression": "10 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r2", "expression": "5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "15 mm"}, nil)

	// Bottom circle on XY.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "1 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{c0[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{c0[0]}, "expression": "r1"}, nil)
	requireDOF(t, cs, 0)

	// Top circle on a work plane offset h above XY (offset is parameter-driven).
	var wp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "h"}, &wp)
	var sk1 struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk1)
	c1 := idsOf(t, cs, map[string]any{"sketchIndex": sk1.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.5 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sk1.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sk1.SketchIndex, "kind": "radius", "entities": []uint64{c1[0]}, "expression": "r2"}, nil)
	requireDOF(t, cs, sk1.SketchIndex)

	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": []map[string]any{
			{"sketchIndex": 0, "profileIndex": 0},
			{"sketchIndex": sk1.SketchIndex, "profileIndex": 0},
		},
	}); !healthy {
		t.Fatalf("loft unhealthy: %s", reason)
	}

	want := func(r1MM, r2MM, hMM float64) float64 {
		r1, r2, h := r1MM/10, r2MM/10, hMM/10
		return math.Pi * h / 3 * (r1*r1 + r1*r2 + r2*r2)
	}
	if got, w := partVolume(t, cs), want(10, 5, 15); math.Abs(got-w)/w > 0.03 {
		t.Errorf("frustum volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Resize the HEIGHT parameter: the offset work plane moves and the top sketch
	// tracks it, so the frustum gets taller — the sketch-on-work-plane associativity.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "h", "expression": "25 mm"}, nil)
	if got, w := partVolume(t, cs), want(10, 5, 25); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized (taller) frustum volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
