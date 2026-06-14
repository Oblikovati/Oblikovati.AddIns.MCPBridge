// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopFixingBlock models NopSCADlib's fixing_block (printed/fixing_block.scad) the Inventor
// way and is the corpus's first test of the MODEL MIRROR FEATURE. A fixing block joins two
// sheets at right angles: a rectangular block with TWO vertical screw bores that are a mirror
// pair about the block's centre plane, plus one horizontal bore on that centre plane.
//
// The Inventor build: extrude the block, cut ONE vertical bore off-centre with a sketched
// circle, then MIRROR that cut feature across the YZ centre plane to make the second bore — so
// the mirror reproduces a sketched cut (a curved tool) and the volume proves both bores exist.
// The horizontal bore sits on the mirror plane, so it is a single centred drill (no mirror).
//
// Faithful simplification: NopSCADlib rounds the block corners (circle4n hull) and sizes the
// bores/pitch from the M3 insert table; we use clean dimensions and an exact analytic volume,
// preserving the mirror structure (block + symmetric bore pair + centred cross bore).
//
// Reference: NopSCADlib/printed/fixing_block.scad (fixing_block_v_holes = mirror pair).
func TestNopFixingBlock(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"W", "24 mm"}, {"D", "12 mm"}, {"th", "12 mm"}, {"vd", "4 mm"}} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}

	// Block profile: a W×D rectangle, corner at the origin (so the centre plane is x = W/2).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	block := rectFull(t, cs, [][]float64{{0, 0}, {2.4, 1.2}})
	bl, br, _, tl := block.points[0], block.points[1], block.points[2], block.points[3]
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	dim := func(sketch int, kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketch, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	con("ground", bl)
	con("horizontal", bl, br)
	con("vertical", bl, tl)
	con("horizontal", block.points[3], block.points[2]) // top edge
	con("vertical", br, block.points[2])                // right edge
	dim(0, "distance", "W", bl, br)
	dim(0, "distance", "D", bl, tl)
	requireConstrained(t, cs, 0)

	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": 0, "profileIndex": closedProfileIndex(t, cs), "distance": "th", "operation": "new",
	}}, nil)

	// Seed vertical bore: a circle at x = W/2 − pitch/2 (= 6 mm), centred in depth, cut up
	// through the block. Its centre is grounded (position fixed) + a radius dimension → 0 DOF.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	seed := idsOf(t, cs, map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{0.6, 0.6}}, "radius": "0.2 cm"})
	seedE, seedC := seed[0], seed[1]
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 1, "kind": "ground", "entities": []uint64{seedC}}, nil)
	dim(1, "diameter", "vd", seedE)
	requireConstrained(t, cs, 1)

	boreName, healthy, reason := addNamedFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all",
	})
	if !healthy {
		t.Fatalf("vertical bore cut unhealthy: %s", reason)
	}

	// THE MIRROR: reflect the bore cut across the YZ centre plane (x = W/2 = 1.2 cm) → the
	// second bore. If the mirror were a no-op the volume below would carry only one bore.
	if healthy, reason := applyFeature(t, cs, "mirror", map[string]any{
		"sourceFeatures": []string{boreName}, "origin": []float64{1.2, 0, 0}, "normal": []float64{1, 0, 0},
	}); !healthy {
		t.Fatalf("mirror unhealthy: %s", reason)
	}

	// Horizontal cross bore: drilled into the front (min-Y) face — its centroid sits on the
	// centre plane, so this one is a single centred hole, not mirrored.
	if healthy, reason := applyFeature(t, cs, "hole", map[string]any{
		"faceRef": frontFaceKey(t, cs), "diameter": "vd",
	}); !healthy {
		t.Fatalf("horizontal hole unhealthy: %s", reason)
	}

	if got, w := partVolume(t, cs), fixingBlockVolume(12); math.Abs(got-w)/w > 0.02 {
		t.Errorf("fixing block volume = %.6f cm^3, want ~%.6f (mirror may not have made the 2nd bore)", got, w)
	}
	// Parametric: thicken the block → both vertical through bores grow, the cross bore (length
	// D) does not. Exercises recompute of the mirror under a parameter edit.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "th", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), fixingBlockVolume(16); math.Abs(got-w)/w > 0.02 {
		t.Errorf("thickened fixing block volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// requireConstrained asserts a sketch solves to 0 DOF (the plan's fully-constrained gate).
func requireConstrained(t *testing.T, cs *mcp.ClientSession, sketchIndex int) {
	t.Helper()
	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": sketchIndex}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("sketch %d not fully constrained: dof=%d, want 0", sketchIndex, solve.DOF)
	}
}

// fixingBlockVolume = block − two vertical through bores (height th) − one horizontal through
// bore (length D), all disjoint. cm^3; th in mm, fixed W=24 D=12 vd=4.
func fixingBlockVolume(thMM float64) float64 {
	const W, D, r = 2.4, 1.2, 0.2
	th := thMM / 10
	return W*D*th - 2*math.Pi*r*r*th - math.Pi*r*r*D
}

// frontFaceKey returns the reference key of the front face — the planar face whose
// representative point has the smallest Y (the y=0 face a horizontal bore drills into).
func frontFaceKey(t *testing.T, cs *mcp.ClientSession) string {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		t.Fatal("get_reference_keys returned no bodies")
	}
	best, bestY := "", math.Inf(1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[1] < bestY {
			best, bestY = f.Key, f.Point[1]
		}
	}
	if best == "" {
		t.Fatal("no face carried a representative point")
	}
	return best
}
