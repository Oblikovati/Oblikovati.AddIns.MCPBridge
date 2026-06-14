// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// holeProfileAt returns the index of sketch sketchIndex's annular (holed) profile, or -1.
func holeProfileAt(t *testing.T, cs *mcp.ClientSession, sketchIndex int) int {
	t.Helper()
	var p struct {
		Profiles []struct {
			Index int `json:"index"`
			Holes int `json:"holes"`
		} `json:"profiles"`
	}
	callStructured(t, cs, "list_sketch_profiles", map[string]any{"sketchIndex": sketchIndex}, &p)
	for _, pr := range p.Profiles {
		if pr.Holes > 0 {
			return pr.Index
		}
	}
	return -1
}

// annulusSketch draws two concentric circles on sketch sketchIndex and fully constrains them,
// returning the annular profile index. The first circle is grounded; the second is coincident
// to it and both radii are dimensioned to parameters.
func annulusSketch(t *testing.T, cs *mcp.ClientSession, sketchIndex int, outerR, innerR, roParam, riParam string) int {
	t.Helper()
	oc := idsOf(t, cs, map[string]any{"sketchIndex": sketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": outerR})
	ic := idsOf(t, cs, map[string]any{"sketchIndex": sketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": innerR})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "ground", "entities": []uint64{oc[1]}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "coincident", "entities": []uint64{oc[1], ic[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIndex, "kind": "radius", "entities": []uint64{oc[0]}, "expression": roParam}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIndex, "kind": "radius", "entities": []uint64{ic[0]}, "expression": riParam}, nil)
	requireDOF(t, cs, sketchIndex)
	h := holeProfileAt(t, cs, sketchIndex)
	if h < 0 {
		t.Fatalf("sketch %d has no annular profile", sketchIndex)
	}
	return h
}

// TestNopLoftPipe models a tapered hollow pipe (nozzle/reducer): a loft between two ANNULUS
// sections on parallel planes. It is the integration-layer guard for the loft's direct tube
// meshing — a watertight bore rather than a filled cone — over the same router path the live
// app uses. Volume = outer cone frustum − inner cone frustum.
func TestNopLoftPipe(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ro1", "expression": "20 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ri1", "expression": "15 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ro2", "expression": "14 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ri2", "expression": "10 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "40 mm"}, nil)

	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	p0 := annulusSketch(t, cs, 0, "2 cm", "1.5 cm", "ro1", "ri1")

	var wp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "h"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	p1 := annulusSketch(t, cs, sk.SketchIndex, "1.4 cm", "1.0 cm", "ro2", "ri2")

	if healthy, reason := applyFeature(t, cs, "loft", map[string]any{
		"sections": []map[string]any{
			{"sketchIndex": 0, "profileIndex": p0},
			{"sketchIndex": sk.SketchIndex, "profileIndex": p1},
		},
	}); !healthy {
		t.Fatalf("loft pipe unhealthy: %s", reason)
	}

	cone := func(rr, r, hh float64) float64 { return math.Pi * hh / 3 * (rr*rr + rr*r + r*r) }
	pipe := func(ro1, ri1, ro2, ri2, hMM float64) float64 {
		h := hMM / 10
		return cone(ro1/10, ro2/10, h) - cone(ri1/10, ri2/10, h)
	}
	if got, w := partVolume(t, cs), pipe(20, 15, 14, 10, 40); math.Abs(got-w)/w > 0.05 {
		t.Fatalf("tapered pipe volume = %.6f cm^3, want ~%.6f (hollow tube, not a filled cone)", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "h", "expression": "60 mm"}, nil)
	if got, w := partVolume(t, cs), pipe(20, 15, 14, 10, 60); math.Abs(got-w)/w > 0.05 {
		t.Errorf("resized pipe volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
