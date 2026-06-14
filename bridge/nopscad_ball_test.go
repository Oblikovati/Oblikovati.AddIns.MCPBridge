// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopBearingBallParametric models a NopSCADlib bearing ball: a sphere built by
// revolving a half-disk 360° about its diameter. The half-disk is a vertical
// diameter line on the axis plus a semicircular arc; it is fully constrained (0 DOF)
// via ground + midpoint + a single diameter dimension (no arc-radius dim, which the
// API lacks — the radius falls out of the geometry). Stresses curved-body bounds (a
// sphere has no silhouette edge) and revolving an arc profile.
//
// Reference: NopSCADlib/vitamins/ball_bearing.scad (bearing_ball ≈ sphere(d)).
func TestNopBearingBallParametric(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "d", "expression": "5 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	line := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0.25}, {0, -0.25}}})
	lineE, top, bot := line[0], line[1], line[2]
	// Semicircle bulging into +X: center origin, start top, end bottom, clockwise.
	arc := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "arc",
		"points": [][]float64{{0, 0}, {0, 0.25}, {0, -0.25}}, "ccw": false})
	arcCenter, arcStart, arcEnd := arc[1], arc[2], arc[3]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", o)
	con("coincident", arcCenter, o)
	con("coincident", arcStart, top)
	con("coincident", arcEnd, bot)
	con("vertical", top, bot)
	con("midpoint", o, lineE)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{
		"sketchIndex": 0, "kind": "distance", "entities": []uint64{top, bot}, "expression": "d",
	}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("ball sketch not fully constrained: dof=%d, want 0", solve.DOF)
	}

	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}

	// Sphere volume = pi*d^3/6 (cm); curved-faceting band.
	want := func(dMM float64) float64 { dc := dMM / 10; return math.Pi * dc * dc * dc / 6 }
	if got, w := partVolume(t, cs), want(5); math.Abs(got-w)/w > 0.03 {
		t.Errorf("ball volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "d", "expression": "8 mm"}, nil)
	if got, w := partVolume(t, cs), want(8); math.Abs(got-w)/w > 0.03 {
		t.Errorf("resized ball volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
