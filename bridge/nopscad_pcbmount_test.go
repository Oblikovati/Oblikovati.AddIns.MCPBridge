// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopPcbMount models the essence of NopSCADlib's pcb_mount (printed/pcb_mount.scad) — a
// base plate carrying standoff posts at the PCB's screw positions — and is the corpus's first
// test of a JOIN PATTERN: every pattern verified so far cut material (holes/slots); here each
// patterned occurrence ADDS a post (the replicate path that re-applies a Join tool, not a Cut).
//
// The Inventor build: extrude the base, JOIN one corner post, then a 2×2 rectangular pattern of
// that post feature → four posts. The volume proves all four were added: base + 4 posts.
//
// Faithful simplification: NopSCADlib's base is a cross-frame with retaining walls and the
// posts are bored rings; we use a solid base plate and solid posts, preserving the structure a
// pcb mount is (a plate + a pattern of standoffs) and giving an exact analytic volume.
//
// Reference: NopSCADlib/printed/pcb_mount.scad (pcb_mount_screw_positions → pillars).
func TestNopPcbMount(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"L", "40 mm"}, {"W", "30 mm"}, {"bt", "3 mm"}, {"pr", "3 mm"}, {"postLen", "11 mm"}} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}

	// Base plate: an L×W rectangle, corner at the origin, extruded bt.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	base := rectFull(t, cs, [][]float64{{0, 0}, {4, 3}})
	bl, br, tr, tl := base.points[0], base.points[1], base.points[2], base.points[3]
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", bl)
	con("horizontal", bl, br)
	con("vertical", bl, tl)
	con("horizontal", tl, tr)
	con("vertical", br, tr)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, br}, "expression": "L"}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": []uint64{bl, tl}, "expression": "W"}, nil)
	requireConstrained(t, cs, 0)
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": 0, "profileIndex": closedProfileIndex(t, cs), "distance": "bt", "operation": "new",
	}}, nil)

	// Seed post at the (0.5,0.5) corner, JOINED, standing postLen tall (it pokes through the
	// base and rises above it — the union adds only the part above the plate).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	post := idsOf(t, cs, map[string]any{"sketchIndex": 1, "kind": "circle", "points": [][]float64{{0.5, 0.5}}, "radius": "0.3 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 1, "kind": "ground", "entities": []uint64{post[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 1, "kind": "diameter", "entities": []uint64{post[0]}, "expression": "2*pr"}, nil)
	requireConstrained(t, cs, 1)
	postName, healthy, reason := addNamedFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 1, "profileIndex": 0, "operation": "join", "distance": "postLen",
	})
	if !healthy {
		t.Fatalf("seed post unhealthy: %s", reason)
	}

	// THE JOIN PATTERN: a 2×2 grid of the post (steps = L−2·inset, W−2·inset = 3 cm, 2 cm) →
	// four corner posts. If the pattern dropped the Join occurrences the volume would carry
	// only the seed post.
	if healthy, reason := applyFeature(t, cs, "patternRectangular", map[string]any{
		"sourceFeatures": []string{postName}, "countX": 2, "countY": 2,
		"stepX": []float64{3, 0, 0}, "stepY": []float64{0, 2, 0},
	}); !healthy {
		t.Fatalf("rectangular pattern unhealthy: %s", reason)
	}

	if got, w := partVolume(t, cs), pcbMountVolume(11); math.Abs(got-w)/w > 0.02 {
		t.Errorf("pcb mount volume = %.6f cm^3, want ~%.6f (pattern may have dropped Join occurrences)", got, w)
	}
	// Parametric: taller posts → all four grow (the pattern re-applies the same Join tool).
	callJSON(t, cs, "set_parameter", map[string]any{"name": "postLen", "expression": "14 mm"}, nil)
	if got, w := partVolume(t, cs), pcbMountVolume(14); math.Abs(got-w)/w > 0.02 {
		t.Errorf("taller pcb mount volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// pcbMountVolume = base plate + four posts' material ABOVE the plate, cm^3 (postLen in mm,
// fixed L=40 W=30 bt=3 pr=3). The post's lower bt is inside the plate (union, not double-counted).
func pcbMountVolume(postLenMM float64) float64 {
	const L, W, bt, r = 4.0, 3.0, 0.3, 0.3
	above := postLenMM/10 - bt
	return L*W*bt + 4*math.Pi*r*r*above
}
