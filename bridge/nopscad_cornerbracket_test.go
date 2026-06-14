// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopCornerBracket models a flat L corner bracket / gusset the Inventor way: its L outline
// (a concave, non-regular profile that had no clean authoring path before) is drawn with the
// polyline entity, extruded, then a mounting hole is cut through each arm. Exercises polyline →
// a real extrudable CONCAVE profile plus two hole cuts. Volume = L-area·t − two holes.
//
// The two holes also exercise the earcut concave-outer + multi-hole fix (a concave face with
// ≥2 holes used to tessellate to the wrong area → wrong volume; see kernel/ops
// TestEarcutConcaveOuterTwoHoles).
func TestNopCornerBracket(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	const t0 = 0.5 // thickness (cm)

	// L outline (cm): 4×4 with a 2.5×2.5 notch ⇒ two 1.5-wide arms; reflex corner at (1.5,1.5).
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": s0, "kind": "polyline", "closed": true,
		"points": [][]float64{{0, 0}, {4, 0}, {4, 1.5}, {1.5, 1.5}, {1.5, 4}, {0, 4}},
	}, nil)
	if closedProfileIndex(t, cs) < 0 {
		t.Fatal("polyline L did not form a closed profile")
	}
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "5 mm", "operation": "new"})

	// A Ø6 mm mounting hole through each arm.
	s1 := addSketchOn(t, cs)
	for _, c := range [][2]float64{{3, 0.75}, {0.75, 3}} {
		callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "circle", "points": [][]float64{{c[0], c[1]}}, "radius": "3 mm"}, nil)
	}
	for pi := 0; pi < 2; pi++ {
		applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s1, "profileIndex": pi, "distance": "10 mm", "operation": "cut"})
	}

	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 || !ops.Validate(bodies[0]).Valid {
		t.Fatalf("corner bracket is not a single valid solid (bodies=%d)", len(bodies))
	}

	lArea := 4.0*4.0 - 2.5*2.5
	want := lArea*t0 - 2*math.Pi*0.3*0.3*t0
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.02 {
		t.Errorf("corner-bracket volume = %.5f cm^3, want ~%.5f", got, want)
	}
}
