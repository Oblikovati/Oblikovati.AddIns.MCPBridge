// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopVeroboardHoleGrid models a veroboard / perfboard (NopSCADlib vitamins/veroboard.scad):
// a thin substrate drilled with a regular grid of holes. It is a deliberate stress on two
// places small bugs compound on bigger models: (1) a face carrying MANY holes (earcut
// multi-hole tessellation — a past defect that halved areas), and (2) a long chain of booleans
// (a 10×5 = 50-occurrence rectangular pattern of a hole cut). The board must stay a valid
// manifold solid and the volume must equal substrate − 50 holes.
//
// In-proc so it can run ops.Validate after the patterned cut.
func TestNopVeroboardHoleGrid(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	const holes, strips = 6, 4
	const pitch = 0.254 // cm (2.54 mm)

	// Substrate: 25.4 × 12.7 × 1.6 mm.
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {1.27, 0.635}}}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "1.6 mm", "operation": "new"})

	// Seed hole at the grid's lower-left (centered grid), Ø1 mm, then a 10×5 pattern.
	x0 := -float64(holes-1) / 2 * pitch
	y0 := -float64(strips-1) / 2 * pitch
	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "circle", "points": [][]float64{{x0, y0}}, "radius": "0.5 mm"}, nil)
	seed, healthy, reason := addNamedFeature(t, cs, "extrude", map[string]any{"sketchIndex": s1, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
	if !healthy {
		t.Fatalf("seed hole cut unhealthy: %s", reason)
	}
	if h, reason := applyFeature(t, cs, "patternRectangular", map[string]any{
		"sourceFeatures": []string{seed},
		"countX":         holes, "stepX": []float64{pitch, 0, 0},
		"countY": strips, "stepY": []float64{0, pitch, 0},
	}); !h {
		t.Fatalf("hole-grid pattern unhealthy: %s", reason)
	}

	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 {
		t.Fatalf("want 1 body, got %d", len(bodies))
	}
	if r := ops.Validate(bodies[0]); !r.Valid {
		t.Fatalf("veroboard INVALID (manifold=%v closed=%v orient=%v): %v",
			r.Manifold, r.Closed, r.OrientationOK, capIssues(r.Issues))
	}

	// Volume = substrate − (holes·strips) cylinders (faceted Ø1 holes ⇒ band).
	const L, W, th, r = 2.54, 1.27, 0.16, 0.05 // cm
	want := L*W*th - float64(holes*strips)*math.Pi*r*r*th
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.02 {
		t.Errorf("veroboard volume = %.6f cm^3, want ~%.6f (2%% band) — a wrong %d-hole face area compounds here", got, want, holes*strips)
	}
}
