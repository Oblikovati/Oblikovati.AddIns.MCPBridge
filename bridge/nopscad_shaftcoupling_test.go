// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopShaftCouplingRadialHoles models a rigid shaft coupling (NopSCADlib
// vitamins/shaft_coupling.scad, SC_5x8_rigid: length 25, OD 12.5, bores Ø5/Ø8) the
// Inventor way: a faceted cylinder, a stepped through bore, and four radial M3 grub-screw
// holes that pierce the wall INTO the bore cavity. The grub holes are the point of this
// part — each is a tool partially penetrating the (re-entrant) faceted bore wall, the
// exact configuration that was feared to need the curved/partial-penetration boolean fix.
// It does not: an axis-perpendicular radial hole through a faceted bore is the well-behaved
// case (unlike an oblique tool grazing a re-entrant CORNER), so every boolean stays a valid
// manifold solid. This pins that.
//
// In-proc (not over the wire) so it can run the kernel validator (ops.Validate: manifold,
// closed, oriented) on the live body after every boolean — the property that matters here.
func TestNopShaftCouplingRadialHoles(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	mustValid := func(step string) {
		t.Helper()
		part, err := modelaccess.ActivePart(s)
		if err != nil {
			t.Fatalf("%s: active part: %v", step, err)
		}
		bodies := part.SurfaceBodies().All()
		if len(bodies) != 1 {
			t.Fatalf("%s: want 1 body, got %d", step, len(bodies))
		}
		if r := ops.Validate(bodies[0]); !r.Valid {
			t.Fatalf("%s: body INVALID (manifold=%v closed=%v orient=%v): %v",
				step, r.Manifold, r.Closed, r.OrientationOK, capIssues(r.Issues))
		}
	}

	// Solid cylinder: Ø12.5 × 25 mm (symmetric about XY).
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "6.25 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "25 mm", "operation": "new", "direction": "symmetric"})
	mustValid("cylinder")

	// Stepped bore: Ø5 lower half (down −Z) meeting Ø8 upper half (up +Z) at z=0.
	sLo := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sLo, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2.5 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sLo, "profileIndex": 0, "distance": "12.5 mm", "operation": "cut", "direction": "negative"})
	sHi := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sHi, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "4 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sHi, "profileIndex": 0, "distance": "12.5 mm", "operation": "cut", "direction": "positive"})
	mustValid("stepped-bore")

	// The stepped tube volume (faceted circles ⇒ ~1–2% under the analytic ideal).
	tube := func(odMM float64) float64 {
		R, r1, r2, half := odMM/20, 2.5/10, 4.0/10, 25.0/20 // mm → cm; each half = 12.5 mm
		return math.Pi*R*R*(2*half) - math.Pi*r1*r1*half - math.Pi*r2*r2*half
	}
	tubeVol := partVolume(t, cs)
	if w := tube(12.5); math.Abs(tubeVol-w)/w > 0.03 {
		t.Errorf("stepped-tube volume = %.5f cm^3, want ~%.5f (3%% faceting band)", tubeVol, w)
	}

	// Four radial M3 grub holes (Ø3) at z=±7.5 mm, along X (sketch on YZ) and Y (sketch on
	// XZ): each pierces the wall into the bore. Validity after every cut is the assertion.
	grub := func(plane string, zCM float64, tag string) {
		var sk struct {
			SketchIndex int `json:"sketchIndex"`
		}
		callJSON(t, cs, "create_sketch", map[string]any{"plane": plane}, &sk)
		callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "circle", "points": [][]float64{{0, zCM}}, "radius": "1.5 mm"}, nil)
		if h, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": "20 mm", "operation": "cut", "direction": "symmetric"}); !h {
			t.Fatalf("grub %s cut unhealthy: %s", tag, reason)
		}
		mustValid("grub-" + tag)
	}
	grub("YZ", 0.75, "X+z")
	grub("XZ", -0.75, "Y-z")
	grub("YZ", -0.75, "X-z")
	grub("XZ", 0.75, "Y+z")

	// The grub holes removed wall material: a valid solid strictly lighter than the tube.
	if withGrubs := partVolume(t, cs); withGrubs >= tubeVol {
		t.Errorf("grub holes removed no material: %.5f >= tube %.5f", withGrubs, tubeVol)
	}
}
