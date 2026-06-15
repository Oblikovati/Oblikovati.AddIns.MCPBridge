// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/app"
)

// TestNopTwinBoreBoss models a NopSCADlib-style twin-bore boss (two parallel cable/pillar bosses
// merged side by side) reduced to the boolean that stresses the kernel: two equal cylinders, axes
// PARALLEL and close enough to overlap, unioned into a figure-8 solid. The seam is two nearly
// straight edges running the full length where the cylinder walls cross — a grazing, near-tangent
// curved-on-curved union, the same family that exposed the tangent-edge boolean defects
// (kernel #871). It complements the ball stud (circular seam) and the cross fitting (saddle seam).
//
// Exact oracle: with centres distance d < 2r apart the two circular sections overlap in a
// symmetric lens of area 2r²·acos(d/2r) − (d/2)·√(4r²−d²); extruded over length L that lens is the
// double-counted volume, so the union is 2·π·r²·L − A_lens·L. A mis-stitched seam misses it.
func TestNopTwinBoreBoss(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	newPartDoc(t, cs, "twin_bore.obk")

	const dCm = 0.7              // centre-to-centre distance (cm); < 2r so the bosses overlap
	b.param("boss_d", "10 mm")   // r = 0.5 cm
	b.param("boss_len", "20 mm") // L = 2 cm

	// 1. First boss: a cylinder along Z, its axis offset −d/2 in X.
	skA := addSketchOnPlane(t, cs, "XY")
	b.dim(skA, "diameter", "boss_d", b.circle(skA, -dCm/2, 0, "0.5 cm")[0])
	b.solved(skA)
	b.feat("1-boss-a", "extrude", map[string]any{"sketchIndex": skA, "profileIndex": 0, "distance": "boss_len", "operation": "new"})

	// 2. Second boss: same cylinder offset +d/2 in X, JOINED — the parallel overlap.
	skB := addSketchOnPlane(t, cs, "XY")
	b.dim(skB, "diameter", "boss_d", b.circle(skB, dCm/2, 0, "0.5 cm")[0])
	b.solved(skB)
	b.feat("2-boss-b", "extrude", map[string]any{"sketchIndex": skB, "profileIndex": 0, "distance": "boss_len", "operation": "join"})

	wantVol := func(dia, lenMM float64) float64 {
		r, L := dia/20, lenMM/10 // mm -> cm (diameter halved)
		lens := 2*r*r*math.Acos(dCm/(2*r)) - (dCm/2)*math.Sqrt(4*r*r-dCm*dCm)
		return 2*math.Pi*r*r*L - lens*L
	}
	if got, w := partVolume(t, cs), wantVol(10, 20); math.Abs(got-w)/w > 0.03 {
		t.Errorf("twin-bore volume = %.6f cm^3, want ~%.6f (lens union, 3%% faceting band)", got, w)
	}

	// Parametric resize: grow the bosses (more overlap at the fixed centre distance) and confirm
	// the figure-8 union rebuilds to the new closed form.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "boss_d", "expression": "12 mm"}, nil)
	b.mustValid("resized")
	if got, w := partVolume(t, cs), wantVol(12, 20); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized twin-bore volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
