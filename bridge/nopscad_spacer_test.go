// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopSpacerParametric models a NopSCADlib-style spacer/standoff: a tube formed
// by extruding the annular region between two concentric circles. It fully
// constrains the sketch (0 DOF) with the centers grounded and radii driven by
// od/id parameters, extrudes to height h, checks the analytic tube volume, then
// resizes. Exercises circle-center grounding, the annular profile, and extrude.
func TestNopSpacerParametric(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "od", "expression": "6 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "id", "expression": "3.2 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "10 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	outer := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.3 cm"})
	inner := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.16 cm"})
	outerE, outerC := outer[0], outer[1]
	innerE, innerC := inner[0], inner[1]

	call := func(name string, args map[string]any) {
		callJSON(t, cs, name, args, nil)
	}
	call("add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{outerC}})
	call("add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "coincident", "entities": []uint64{outerC, innerC}})
	call("add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{outerE}, "expression": "od / 2"})
	call("add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "radius", "entities": []uint64{innerE}, "expression": "id / 2"})

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("spacer sketch not fully constrained: dof=%d", solve.DOF)
	}

	annulus := profileWithHole(t, cs)
	if annulus < 0 {
		t.Fatal("no annular profile (outer ring with inner hole) found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": annulus, "distance": "h"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	want := func(odMM, idMM, hMM float64) float64 {
		R, r, h := odMM/20, idMM/20, hMM/10
		return math.Pi * (R*R - r*r) * h
	}
	if got, w := partVolume(t, cs), want(6, 3.2, 10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("spacer volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "od", "expression": "8 mm"}, nil)
	if got, w := partVolume(t, cs), want(8, 3.2, 10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("resized spacer volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// profileWithHole returns the index of the first sketch profile that has an inner
// loop (a hole) — the annular region — or -1.
func profileWithHole(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	var p struct {
		Profiles []struct {
			Index int `json:"index"`
			Holes int `json:"holes"`
		} `json:"profiles"`
	}
	callStructured(t, cs, "list_sketch_profiles", map[string]any{"sketchIndex": 0}, &p)
	for _, pr := range p.Profiles {
		if pr.Holes > 0 {
			return pr.Index
		}
	}
	return -1
}
