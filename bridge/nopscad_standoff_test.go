// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopStandoffCapsule models the rounded pin of a NopSCADlib standoff: hull(sphere, sphere)
// — two balls a distance apart wrapped into a capsule. It is the convex-hull op working on
// fully CURVED bodies (each ball is a revolved half-disk, a separate body), then hulled. The
// capsule volume is a cylinder plus a full sphere: πr²L + (4/3)πr³. Spreading the balls (a
// parameter edit) lengthens the capsule.
//
// Reference: NopSCADlib/vitamins/pcb.scad standoff() — hull() of two sphere(d2).
func TestNopStandoffCapsule(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "d2", "expression": "4 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "L", "expression": "10 mm"}, nil)

	ball(t, cs, 0, "")  // first ball centred at the origin
	ball(t, cs, 1, "L") // second ball L up the Y axis

	if healthy, reason := applyFeature(t, cs, "hull", map[string]any{}); !healthy {
		t.Fatalf("hull unhealthy: %s", reason)
	}
	if n := bodyCount(t, cs); n != 1 {
		t.Fatalf("hull should leave 1 body, got %d", n)
	}

	// Capsule volume = πr²L + (4/3)πr³, cm^3 (r = d2/2).
	wantVol := func(d2MM, lMM float64) float64 {
		r, l := d2MM/20, lMM/10
		return math.Pi*r*r*l + (4.0/3.0)*math.Pi*r*r*r
	}
	if got, w := partVolume(t, cs), wantVol(4, 10); math.Abs(got-w)/w > 0.05 {
		t.Errorf("standoff capsule volume = %.6f cm^3, want ~%.6f (5%% curved band)", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "L", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(4, 16); math.Abs(got-w)/w > 0.05 {
		t.Errorf("spread standoff capsule volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// ball builds a fully-constrained sphere (a half-disk revolved 360° about Y) whose centre sits
// at y = centreExpr up the axis (centreExpr "" ⇒ at the origin), as a separate body.
func ball(t *testing.T, cs *mcp.ClientSession, sketchIdx int, centreExpr string) {
	t.Helper()
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	o := idsOf(t, cs, map[string]any{"sketchIndex": sketchIdx, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	cy := 0.0
	if centreExpr != "" {
		cy = 1.0
	}
	line := idsOf(t, cs, map[string]any{"sketchIndex": sketchIdx, "kind": "line",
		"points": [][]float64{{0, cy + 0.2}, {0, cy - 0.2}}})
	lineE, top, bot := line[0], line[1], line[2]
	arc := idsOf(t, cs, map[string]any{"sketchIndex": sketchIdx, "kind": "arc", "ccw": false,
		"points": [][]float64{{0, cy}, {0, cy + 0.2}, {0, cy - 0.2}}})
	arcCenter, arcStart, arcEnd := arc[1], arc[2], arc[3]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIdx, "kind": kind, "entities": ents}, nil)
	}
	con("ground", o)
	con("coincident", arcStart, top)
	con("coincident", arcEnd, bot)
	con("vertical", top, bot)
	con("midpoint", arcCenter, lineE)
	if centreExpr == "" {
		con("coincident", arcCenter, o)
	} else {
		con("vertical", o, arcCenter)
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIdx, "kind": "distance", "entities": []uint64{o, arcCenter}, "expression": centreExpr}, nil)
	}
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIdx, "kind": "distance", "entities": []uint64{top, bot}, "expression": "d2"}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": sketchIdx}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("ball %d sketch not fully constrained: dof=%d", sketchIdx, solve.DOF)
	}
	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": sketchIdx, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg", "operation": "new",
	}); !healthy {
		t.Fatalf("ball %d revolve unhealthy: %s", sketchIdx, reason)
	}
}
