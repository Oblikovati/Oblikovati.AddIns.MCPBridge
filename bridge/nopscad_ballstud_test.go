// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

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

	// Volume sanity: the union is the sphere plus the part of the shank that sticks out past it.
	// The shank emerges from the sphere at y = sqrt(r_ball² − r_shank²); beyond that it is a free
	// cylinder of length (shank_len − that), so V = sphere + π·r_shank²·(shank_len − y_emerge).
	wantVol := func(ballMM, shankMM, lenMM float64) float64 {
		rB, rS, L := ballMM/20, shankMM/20, lenMM/10 // mm -> cm (diameters halved)
		yEmerge := math.Sqrt(rB*rB - rS*rS)
		sphere := math.Pi * rB * rB * rB * 4.0 / 3.0
		freeShank := math.Pi * rS * rS * (L - yEmerge)
		return sphere + freeShank
	}
	if got, w := partVolume(t, cs), wantVol(10, 6, 15); math.Abs(got-w)/w > 0.03 {
		t.Errorf("ball stud volume = %.6f cm^3, want ~%.6f (3%% faceting band)", got, w)
	}

	// Parametric resize: grow the head and confirm the union rebuilds and the volume tracks.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "ball_d", "expression": "14 mm"}, nil)
	b.mustValid("resized")
	if got, w := partVolume(t, cs), wantVol(14, 6, 15); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized ball stud volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
