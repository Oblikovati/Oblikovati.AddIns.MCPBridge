// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopHullTwoCylinders exercises the convex-hull feature — the kernel behind OpenSCAD's
// hull(), which NopSCADlib leans on heavily (standoff = hull of a post + domes, rod ends,
// pulley teeth). Two parallel cylinders are built as separate bodies, then hulled into one
// solid: the result is a "stadium" prism (the 2D stadium — a d×2r rectangle capped by two
// semicircles — extruded by the height). Its volume is (πr² + 2rd)·h. Spreading the
// cylinders (a parameter edit) lengthens the slot and grows the volume.
//
// Reference: NopSCADlib hull() idiom (vitamins/pcb.scad standoff, vitamins/rod.scad).
func TestNopHullTwoCylinders(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "r", "expression": "5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "8 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "d", "expression": "12 mm"}, nil)

	cylinder := func(sketchIdx int, cx float64, centreExpr string) {
		callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
		c := idsOf(t, cs, map[string]any{"sketchIndex": sketchIdx, "kind": "circle",
			"points": [][]float64{{cx, 0}}, "radius": "0.5 cm"})
		o := idsOf(t, cs, map[string]any{"sketchIndex": sketchIdx, "kind": "point", "points": [][]float64{{0, 0}}})[0]
		con := func(kind string, ents ...uint64) {
			callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIdx, "kind": kind, "entities": ents}, nil)
		}
		con("ground", o)
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIdx, "kind": "radius", "entities": []uint64{c[0]}, "expression": "r"}, nil)
		if centreExpr == "" {
			con("coincident", o, c[1]) // centred on the origin
		} else {
			con("horizontal", o, c[1]) // centre on the X axis
			callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIdx, "kind": "distance", "entities": []uint64{o, c[1]}, "expression": centreExpr}, nil)
		}
		var solve struct {
			DOF int `json:"dof"`
		}
		callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": sketchIdx}, &solve)
		if solve.DOF != 0 {
			t.Fatalf("cylinder %d sketch not fully constrained: dof=%d", sketchIdx, solve.DOF)
		}
		if healthy, reason := applyFeature(t, cs, "extrude",
			map[string]any{"sketchIndex": sketchIdx, "profileIndex": 0, "distance": "h", "operation": "new"}); !healthy {
			t.Fatalf("cylinder %d extrude unhealthy: %s", sketchIdx, reason)
		}
	}
	cylinder(0, 0, "")    // at the origin
	cylinder(1, 1.2, "d") // at x = d on the +X axis

	if n := bodyCount(t, cs); n != 2 {
		t.Fatalf("expected 2 separate cylinder bodies before hull, got %d", n)
	}
	if healthy, reason := applyFeature(t, cs, "hull", map[string]any{}); !healthy {
		t.Fatalf("hull unhealthy: %s", reason)
	}
	if n := bodyCount(t, cs); n != 1 {
		t.Fatalf("hull should leave 1 body, got %d", n)
	}

	// Stadium prism volume = (πr² + 2rd)·h, cm^3.
	wantVol := func(rMM, hMM, dMM float64) float64 {
		r, hh, dd := rMM/10, hMM/10, dMM/10
		return (math.Pi*r*r + 2*r*dd) * hh
	}
	if got, w := partVolume(t, cs), wantVol(5, 8, 12); math.Abs(got-w)/w > 0.03 {
		t.Errorf("hull volume = %.6f cm^3, want ~%.6f (3%% faceting band)", got, w)
	}
	// Parametric: spread the cylinders further apart; the slot — and volume — grows.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "d", "expression": "20 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(5, 8, 20); math.Abs(got-w)/w > 0.03 {
		t.Errorf("spread hull volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// bodyCount returns how many bodies the active part has (from the model tree).
func bodyCount(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct{} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	return len(rk.Bodies)
}
