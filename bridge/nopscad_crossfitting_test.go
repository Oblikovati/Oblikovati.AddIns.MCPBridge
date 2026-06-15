// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/app"
)

// TestNopCrossFitting models a NopSCADlib-style cross/tee pipe fitting reduced to its hard
// kernel core: two equal cylinders crossing perpendicular at the origin, UNIONED. Their surfaces
// meet along a saddle curve (a cylinder ∩ cylinder seam, not a planar one) — the most demanding
// curved-on-curved boolean, distinct from the ball stud's circular seam.
//
// The union has an EXACT closed form, which makes it a precise oracle for the saddle boolean:
// two equal cylinders (radius r, each at least 2r long so the other passes fully through)
// intersect in the Steinmetz bicylinder of volume 16/3·r³, so by inclusion–exclusion the union
// is 2·π·r²·L − 16/3·r³. If the kernel mis-stitches the saddle seam (double-counts the overlap,
// drops a face, leaves it open) the volume misses that target — `feat`'s manifold/closed gate
// plus this volume check together pin the seam down.
func TestNopCrossFitting(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	newPartDoc(t, cs, "cross_fitting.obk")

	b.param("cyl_d", "10 mm")   // r = 0.5 cm
	b.param("cyl_len", "30 mm") // L = 3 cm ≥ 2r, so each cylinder fully crosses the other

	// 1. Cylinder along Z, centred on the origin (symmetric extrude splits the length each side).
	skZ := addSketchOnPlane(t, cs, "XY")
	b.dim(skZ, "diameter", "cyl_d", b.circle(skZ, 0, 0, "0.5 cm")[0])
	b.solved(skZ)
	b.feat("1-cyl-z", "extrude", map[string]any{"sketchIndex": skZ, "profileIndex": 0, "distance": "cyl_len", "direction": "symmetric", "operation": "new"})

	// 2. Cylinder along X (sketch on YZ, normal X), same size, JOINED — the perpendicular crossing.
	skX := addSketchOnPlane(t, cs, "YZ")
	b.dim(skX, "diameter", "cyl_d", b.circle(skX, 0, 0, "0.5 cm")[0])
	b.solved(skX)
	b.feat("2-cyl-x", "extrude", map[string]any{"sketchIndex": skX, "profileIndex": 0, "distance": "cyl_len", "direction": "symmetric", "operation": "join"})

	// Inclusion–exclusion with the Steinmetz intersection (16/3·r³) for two equal crossing cylinders.
	wantVol := func(dMM, lenMM float64) float64 {
		r, L := dMM/20, lenMM/10 // mm -> cm (diameter halved)
		return 2*math.Pi*r*r*L - 16.0/3.0*r*r*r
	}
	if got, w := partVolume(t, cs), wantVol(10, 30); math.Abs(got-w)/w > 0.03 {
		t.Errorf("cross fitting volume = %.6f cm^3, want ~%.6f (Steinmetz union, 3%% faceting band)", got, w)
	}

	// Parametric resize: thicken both cylinders (one parameter drives both) and confirm the saddle
	// union rebuilds to the new closed form.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "cyl_d", "expression": "14 mm"}, nil)
	b.mustValid("resized")
	if got, w := partVolume(t, cs), wantVol(14, 30); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized cross fitting volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
