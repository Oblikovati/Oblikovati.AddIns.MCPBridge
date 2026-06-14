// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopRibWall exercises the RIB feature — the only part to do so. An open sketch path (a
// line) is thickened by ±thickness/2 into a band and extruded depth along the plane normal,
// making a support wall (the gusset/web of printed brackets). The wall's volume is
// length·thickness·depth. A parameter edit (taller rib) grows it.
//
// Reference: NopSCADlib printed brackets/ribs (the rib feature thickens an open profile into a
// reinforcing wall).
func TestNopRibWall(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ribLen", "expression": "20 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ribTh", "expression": "2 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ribDepth", "expression": "10 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// An open path: a single line the rib thickens.
	line := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0}, {2, 0}}})
	lineP0, lineP1 := line[1], line[2]
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{lineP0}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "horizontal", "entities": []uint64{lineP0, lineP1}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": []uint64{lineP0, lineP1}, "expression": "ribLen"}, nil)
	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("rib path not fully constrained: dof=%d", solve.DOF)
	}

	if healthy, reason := applyFeature(t, cs, "rib", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "thickness": "ribTh", "depth": "ribDepth", "operation": "new",
	}); !healthy {
		t.Fatalf("rib unhealthy: %s", reason)
	}

	// Wall volume = length·thickness·depth, cm^3.
	wantVol := func(lenMM, thMM, depthMM float64) float64 {
		return (lenMM / 10) * (thMM / 10) * (depthMM / 10)
	}
	if got, w := partVolume(t, cs), wantVol(20, 2, 10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("rib volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "ribDepth", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(20, 2, 16); math.Abs(got-w)/w > 0.02 {
		t.Errorf("taller rib volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
