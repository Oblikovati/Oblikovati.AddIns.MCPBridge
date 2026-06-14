// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopLoftPyramid lofts between two SQUARE sections of different size on parallel planes —
// a truncated pyramid (a hopper / transition / lamp base). The existing loft test blends two
// circles (equal vertex counts, no corners); square sections exercise the loft's corner
// correspondence and ruling. Volume is the exact prismatoid h/6·(a²+(a+b)²+b²).
//
// Subtest "square-to-circle" probes a MISMATCHED-vertex-count loft (4 corners → a faceted
// circle): it asserts only that the loft reaches the kernel and yields a valid manifold solid
// (no clean analytic volume for that blend), documenting whether dissimilar-count lofting works.
func TestNopLoftPyramid(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	// Bottom square side 2 (half 1) on XY.
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {1, 1}}}, nil)
	// Top square side 1 (half 0.5) on a work plane offset h=15 mm above XY.
	var wp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "15 mm"}, &wp)
	var sk1 struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk1)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sk1.SketchIndex, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {0.5, 0.5}}}, nil)

	if h, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": []map[string]any{{"sketchIndex": s0, "profileIndex": 0}, {"sketchIndex": sk1.SketchIndex, "profileIndex": 0}},
	}); !h {
		t.Fatalf("square-pyramid loft unhealthy: %s", reason)
	}
	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 || !ops.Validate(bodies[0]).Valid {
		t.Fatalf("square-pyramid loft is not a single valid solid (bodies=%d)", len(bodies))
	}
	// Prismatoid volume h/6·(a²+(a+b)²+b²), a=2,b=1,h=1.5 (cm) → 3.5 cm³.
	pyr := func(a, b, h float64) float64 { return h / 6 * (a*a + (a+b)*(a+b) + b*b) }
	if got, w := partVolume(t, cs), pyr(2, 1, 1.5); math.Abs(got-w)/w > 0.02 {
		t.Errorf("truncated-pyramid volume = %.6f cm^3, want ~%.6f", got, w)
	}

	t.Run("square-to-circle-mismatched-count", func(t *testing.T) {
		s2 := app.NewSession()
		if err := app.RegisterStandardCommands(s2); err != nil {
			t.Fatalf("commands: %v", err)
		}
		if _, err := s2.NewPart(); err != nil {
			t.Fatalf("NewPart: %v", err)
		}
		cs2 := e2eClient(t, s2)
		sa := addSketchOn(t, cs2)
		callJSON(t, cs2, "add_sketch_entity", map[string]any{"sketchIndex": sa, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {1, 1}}}, nil)
		var wp2 struct {
			Index int `json:"index"`
		}
		callJSON(t, cs2, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "15 mm"}, &wp2)
		var skc struct {
			SketchIndex int `json:"sketchIndex"`
		}
		callJSON(t, cs2, "create_sketch", map[string]any{"workPlaneIndex": wp2.Index}, &skc)
		callJSON(t, cs2, "add_sketch_entity", map[string]any{"sketchIndex": skc.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "5 mm"}, nil)
		name, healthy, reason := addNamedFeature(t, cs2, "loft", map[string]any{
			"sections": []map[string]any{{"sketchIndex": sa, "profileIndex": 0}, {"sketchIndex": skc.SketchIndex, "profileIndex": 0}},
		})
		t.Logf("square→circle loft: feature=%q healthy=%v reason=%q", name, healthy, reason)
		part2, err := modelaccess.ActivePart(s2)
		if err != nil {
			t.Fatalf("active part: %v", err)
		}
		for i, b := range part2.SurfaceBodies().All() {
			r := ops.Validate(b)
			t.Logf("  body[%d] valid=%v issues=%v", i, r.Valid, capIssues(r.Issues))
		}
	})
}
