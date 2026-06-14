// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopPcbHoleGrid models a PCB blank: a rectangular board with a 2×2 grid of
// mounting holes whose spacing is parameter-driven (tied to the board size and
// margin). Resizing the board length widens the grid so the holes stay at the
// corners — proving sketch-pattern spacing is parametric. Then it extrudes the
// board-with-holes profile and checks the volume.
//
// Reference: NopSCADlib/vitamins/pcb.scad (a board with a corner hole pattern).
func TestNopPcbHoleGrid(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "L", "expression": "40 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "W", "expression": "30 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "m", "expression": "4 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "hr", "expression": "1.5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "th", "expression": "1.6 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Board rectangle, corner at the origin. Capture all four edge lines + corners.
	board := rectFull(t, cs, [][]float64{{0, 0}, {4, 3}})
	bottom, right, top, left := board.lines[0], board.lines[1], board.lines[2], board.lines[3]
	bl, br, tr, tl := board.points[0], board.points[1], board.points[2], board.points[3]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", bl)
	con("horizontal", bl, br)
	con("horizontal", tl, tr)
	con("vertical", bl, tl)
	con("vertical", br, tr)
	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	dim("distance", "L", bl, br)
	dim("distance", "W", bl, tl)

	// Seed hole near the bottom-left corner: centre m from the bottom and left edges.
	seed := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0.4, 0.4}}, "radius": "0.15 cm"})
	seedE, seedC := seed[0], seed[1]
	dim("offsetDim", "m", seedC, bottom) // distance to the bottom edge = m
	dim("offsetDim", "m", seedC, left)   // distance to the left edge = m
	dim("radius", "hr", seedE)
	_ = right
	_ = top

	// A 2×1 row of holes along +X, spacing tied to the board (L − 2·margin) so the
	// second hole sits at the far edge. The spacing expression references parameters,
	// which is the point: editing L re-spaces the pattern.
	callJSON(t, cs, "add_sketch_pattern", map[string]any{
		"sketchIndex": 0, "kind": "rectangular", "entities": []uint64{seedE},
		"count1": 2, "spacing1": "L - 2*m", "dir1": []float64{1, 0},
		"count2": 1, "spacing2": "1 mm", "dir2": []float64{0, 1},
	}, nil)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("pcb sketch not fully constrained: dof=%d, want 0", solve.DOF)
	}

	// Directly prove parametric spacing: the clone hole sits at x = L − m. Widen L and
	// confirm it tracks (this is the whole point and is fast — no rebuild needed).
	if x := cloneHoleX(t, cs); math.Abs(x-(4.0-0.4)) > 1e-2 {
		t.Fatalf("clone hole X = %.3f, want %.3f (= L − m at L=40mm)", x, 4.0-0.4)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "L", "expression": "60 mm"}, nil)
	if x := cloneHoleX(t, cs); math.Abs(x-(6.0-0.4)) > 1e-2 {
		t.Errorf("after L→60mm, clone hole X = %.3f, want %.3f (spacing did not track)", x, 6.0-0.4)
	}

	// And the board-with-holes extrudes to a valid solid of the expected volume.
	profIdx := profileWithHole(t, cs)
	if profIdx < 0 {
		t.Fatal("no board-with-holes profile found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": profIdx, "distance": "th"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}
	want := func(lMM, wMM, hrMM, thMM float64) float64 {
		l, w, hr, th := lMM/10, wMM/10, hrMM/10, thMM/10
		return (l*w - 2*math.Pi*hr*hr) * th
	}
	if got, w := partVolume(t, cs), want(60, 30, 1.5, 1.6); math.Abs(got-w)/w > 0.02 {
		t.Errorf("pcb volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// cloneHoleX returns the X coordinate of the second circle (the patterned clone) after
// solving the sketch.
func cloneHoleX(t *testing.T, cs *mcp.ClientSession) float64 {
	t.Helper()
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, nil)
	var ents struct {
		Entities []struct {
			Kind   string      `json:"kind"`
			Points [][]float64 `json:"points"`
		} `json:"entities"`
	}
	callStructured(t, cs, "list_sketch_entities", map[string]any{"sketchIndex": 0}, &ents)
	var xs []float64
	for _, e := range ents.Entities {
		if e.Kind == "circle" && len(e.Points) == 1 && len(e.Points[0]) == 2 {
			xs = append(xs, e.Points[0][0])
		}
	}
	if len(xs) < 2 {
		t.Fatalf("expected 2 circles, found %d", len(xs))
	}
	if xs[0] < xs[1] {
		return xs[1]
	}
	return xs[0]
}

// rectComposite holds a rectangle's member line ids and corner point ids.
type rectComposite struct {
	lines  []uint64
	points []uint64
}

// rectFull adds a rectangle and returns its four edge-line ids and four corner ids.
func rectFull(t *testing.T, cs *mcp.ClientSession, pts [][]float64) rectComposite {
	t.Helper()
	var r struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": pts}, &r)
	if len(r.EntityIDs) < 4 || len(r.PointIDs) < 4 {
		t.Fatalf("rectangle reply: lines=%v points=%v", r.EntityIDs, r.PointIDs)
	}
	return rectComposite{lines: r.EntityIDs, points: r.PointIDs}
}
