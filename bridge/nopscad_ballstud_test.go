// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/app"
)

// TestNopBallStud models a NopSCADlib-style ball stud / rod end: a spherical head joined onto a
// cylindrical shank, both coaxial on the Y axis. Unlike the standalone bearing ball
// (TestNopBearingBallParametric), the sphere is UNIONED onto the shank — so the kernel must
// boolean two CURVED bodies whose surfaces meet along a circle (cylinder ∩ sphere). That
// curved-on-curved union is the same class that exposed the tangent/orientation/weld defects
// fixed in the boolean (kernel #871/#877/#880); a ball stud is the smallest faithful part that
// drives it, and `feat` gates every step on one manifold/closed/oriented solid.
//
// It also proves the union stays EXACT. A coaxial sphere ∪ cylinder is the one sphere∩cylinder
// configuration with a closed-form intersection — a circle at ±√(R_s²−R_c²) along the axis, the case
// OCCT solves analytically in IntAna_QuadQuadGeo — so the result must be THREE analytic faces, not the
// ~500-facet inscribed polyhedron the CSG fallback shipped before Oblikovati#2036.
//
// Reference: NopSCADlib rod ends / ball studs (sphere head ⌀ on a shank ⌀).
func TestNopBallStud(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	newPartDoc(t, cs, "ball_stud.obk")

	b.param("ball_d", "10 mm")
	b.param("shank_d", "6 mm")
	b.param("shank_len", "15 mm")

	// 1. Shank: a Ø6 cylinder along +Y (sketch on XZ, whose normal is Y), base at the origin.
	skShank := addSketchOnPlane(t, cs, "XZ")
	b.dim(skShank, "diameter", "shank_d", b.circle(skShank, 0, 0, "0.3 cm")[0])
	b.solved(skShank)
	b.feat("1-shank", "extrude", map[string]any{"sketchIndex": skShank, "profileIndex": 0, "distance": "shank_len", "operation": "new"})

	// 2. Ball: a half-disk (diameter line on the Y axis + a semicircular arc bulging +X) revolved
	// 360° about Y into a sphere centred at the origin, JOINED onto the shank base. The shank
	// (Ø6 < ball Ø10) enters the sphere, so the union seam is a circle on the sphere — curved∩curved.
	skBall := addSketchOnPlane(t, cs, "XY")
	o := idsOf(t, cs, map[string]any{"sketchIndex": skBall, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	line := idsOf(t, cs, map[string]any{"sketchIndex": skBall, "kind": "line", "points": [][]float64{{0, 0.5}, {0, -0.5}}})
	lineE, top, bot := line[0], line[1], line[2]
	arc := idsOf(t, cs, map[string]any{"sketchIndex": skBall, "kind": "arc",
		"points": [][]float64{{0, 0}, {0, 0.5}, {0, -0.5}}, "ccw": false})
	arcCenter, arcStart, arcEnd := arc[1], arc[2], arc[3]
	b.con(skBall, "ground", o)
	b.con(skBall, "coincident", arcCenter, o)
	b.con(skBall, "coincident", arcStart, top)
	b.con(skBall, "coincident", arcEnd, bot)
	b.con(skBall, "vertical", top, bot)
	b.con(skBall, "midpoint", o, lineE)
	b.dim(skBall, "distance", "ball_d", top, bot)
	b.solved(skBall)
	b.feat("2-ball", "revolve", map[string]any{
		"sketchIndex": skBall, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg", "operation": "join",
	})

	if got, w := partVolume(t, cs), ballStudVolume(10, 6, 15); math.Abs(got-w)/w > ballStudBand {
		t.Errorf("ball stud volume = %.6f cm^3, want ~%.6f (%.1f%% band)", got, w, 100*ballStudBand)
	}
	assertBallStudIsAnalytic(t, cs)

	// Parametric resize: grow the head and confirm the union rebuilds, stays analytic, and tracks.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "ball_d", "expression": "14 mm"}, nil)
	b.mustValid("resized")
	if got, w := partVolume(t, cs), ballStudVolume(14, 6, 15); math.Abs(got-w)/w > ballStudBand {
		t.Errorf("resized ball stud volume = %.6f cm^3, want ~%.6f", got, w)
	}
	assertBallStudIsAnalytic(t, cs)
}

// assertBallStudIsAnalytic reads the part's topology over the wire and fails unless the union is the
// exact three-face B-rep: the ball's remaining spherical zone, the shank's cylindrical wall, and its
// end cap. This is the half of Oblikovati#2036 the volume band cannot see on its own — a CSG fallback
// with a lucky volume would still show up here as hundreds of "plane" faces and no sphere.
func assertBallStudIsAnalytic(t *testing.T, cs *mcp.ClientSession) {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Kind string `json:"kind"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) != 1 {
		t.Fatalf("ball stud is %d bodies, want 1", len(rk.Bodies))
	}
	kinds := map[string]int{}
	for _, f := range rk.Bodies[0].Faces {
		kinds[f.Kind]++
	}
	if kinds["sphere"] != 1 || kinds["cylinder"] != 1 || kinds["plane"] != 1 || len(rk.Bodies[0].Faces) != 3 {
		t.Errorf("ball stud has %d faces %v, want one sphere + one cylinder + one plane — "+
			"a faceted result means the union fell back to triangle-soup CSG",
			len(rk.Bodies[0].Faces), kinds)
	}
}

// ballStudBand is how far the measured volume may sit below the exact value. It is now a pure
// TESSELLATION budget: the B-rep is exact (a coaxial sphere ∪ cylinder takes the closed-form circle
// path, kernel/brep curved_coaxial_sphere_rod.go), so all that remains is the inscribed-facet deficit
// of the spherical zone at the mass-properties quality. Before Oblikovati#2036 this had to be 2% to
// accommodate a CSG fallback whose ~1.34% error did NOT shrink with tessellation quality — flat from a
// 4° to a 0.1° angular tolerance — because the deficit was in the B-rep, not in the mesh.
const ballStudBand = 0.001

// ballStudVolume is the EXACT volume of the ball stud: a sphere of diameter ballMM centred at the
// origin, unioned with a coaxial cylinder of diameter shankMM running from the centre out to lenMM.
//
//	V = V(sphere) + V(cylinder ∖ sphere)
//	V(cylinder ∖ sphere) = ∫₀^rS 2πr·(L − √(rB²−r²)) dr
//	                     = 2π·( L·rS²/2 − (rB³ − (rB²−rS²)^{3/2})/3 )
//
// The shank's free length is measured PER RADIUS: at radius r the sphere surface sits at
// y = √(rB²−r²), which runs from rB on the axis down to √(rB²−rS²) at the wall. Treating the shank
// as a full cylinder above the wall's emergence plane y = √(rB²−rS²) — the closed form this test
// asserted against until now — double-counts the annular wedge between that plane and the sphere,
// overstating the ball stud by 1.8% (0.834616 against the true 0.819956 at Ø10/Ø6/15) and by 14%
// on a thick shank. That overstatement, not the kernel, is what pushed this test past its band.
//
// Example: ballStudVolume(10, 6, 15) == 0.819956 cm³.
func ballStudVolume(ballMM, shankMM, lenMM float64) float64 {
	rB, rS, L := ballMM/20, shankMM/20, lenMM/10 // mm -> cm (diameters halved)
	sphere := 4.0 / 3.0 * math.Pi * rB * rB * rB
	freeShank := 2 * math.Pi * (L*rS*rS/2 - (rB*rB*rB-math.Pow(rB*rB-rS*rS, 1.5))/3)
	return sphere + freeShank
}

// TestBallStudVolumeMatchesNumericIntegration pins the closed form against a direct numeric
// integration of the same region, so the double-counting the analytic shortcut invited
// cannot come back unnoticed: the shortcut and the integral disagree by 1.8%,
// far outside this tolerance.
func TestBallStudVolumeMatchesNumericIntegration(t *testing.T) {
	// numeric integrates V = V(sphere) + ∫₀^rS 2πr·(L − √(rB²−r²)) dr by the midpoint rule.
	numeric := func(ballMM, shankMM, lenMM float64) float64 {
		rB, rS, L := ballMM/20, shankMM/20, lenMM/10
		const n = 200000
		free := 0.0
		for i := 0; i < n; i++ {
			r := rS * (float64(i) + 0.5) / n
			free += 2 * math.Pi * r * (L - math.Sqrt(rB*rB-r*r)) * (rS / n)
		}
		return 4.0/3.0*math.Pi*rB*rB*rB + free
	}
	for _, c := range []struct{ ball, shank, length float64 }{
		{10, 6, 15}, {14, 6, 15}, {20, 4, 30}, {8, 7.9, 12},
	} {
		got, want := ballStudVolume(c.ball, c.shank, c.length), numeric(c.ball, c.shank, c.length)
		if math.Abs(got-want)/want > 1e-6 {
			t.Errorf("ballStudVolume(%g,%g,%g) = %.8f, numeric integration = %.8f",
				c.ball, c.shank, c.length, got, want)
		}
	}
}
