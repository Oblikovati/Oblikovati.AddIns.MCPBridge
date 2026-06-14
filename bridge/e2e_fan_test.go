// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A complex, multi-feature modeling test modelled on a parametric PC fan (parametric-fan.scad):
// a square frame with rounded corners, a central bore and hub, a ring of blades (a join feature
// circular-patterned), and four mounting holes. It stresses feature interaction — fillets on
// feature edges, curved cuts, a patterned join, and selection of edges/faces by geometry — and
// checks material was actually added/removed via volume (a missed boolean is a healthy no-op).
//
// (linear_extrude twist for the blades has no direct kernel feature yet — the bridge extrude
// has taper, not twist — so the blades are straight radial fins, the faithful simplification.)
//
// All dimensions are cm (database units): width 5, depth 1.5, bore Ø4.7, hub Ø1.35, corner r
// 0.5, mount span ±2, mount Ø0.34.

// edge is a body edge reference key with its representative point.
type edge struct {
	key   string
	point [3]float64
}

// bodyEdges reads the active body's edges with their representative points.
func bodyEdges(t *testing.T, cs *mcp.ClientSession) []edge {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Edges []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		t.Fatal("no body for reference keys")
	}
	var out []edge
	for _, e := range rk.Bodies[0].Edges {
		if len(e.Point) == 3 {
			out = append(out, edge{e.Key, [3]float64{e.Point[0], e.Point[1], e.Point[2]}})
		}
	}
	return out
}

// addSketchOn creates a new sketch on XY and returns its index.
func addSketchOn(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, &sk)
	return sk.SketchIndex
}

func TestE2EParametricFan(t *testing.T) {
	cs := freshPart(t)

	// 1. Frame: a centred 5×5 square, extruded 1.5 thick.
	s := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2.5, 2.5}}}, nil)
	if h, r := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s, "profileIndex": 0, "distance": "15 mm", "operation": "new"}); !h {
		t.Fatalf("frame extrude unhealthy: %s", r)
	}
	frameVol := partVolume(t, cs) // ~ 5*5*1.5 = 37.5

	// 2. Round the four vertical corner edges (the frame's corners), radius 5 mm.
	var corners []string
	for _, e := range bodyEdges(t, cs) {
		if abs(e.point[0]) > 2.0 && abs(e.point[1]) > 2.0 { // a corner column edge
			corners = append(corners, e.key)
		}
	}
	if len(corners) < 4 {
		t.Fatalf("found %d corner edges, want >= 4", len(corners))
	}
	if h, r := applyFeature(t, cs, "fillet", map[string]any{"edgeRefs": corners[:4], "radius": "5 mm"}); !h {
		t.Fatalf("corner fillet unhealthy: %s", r)
	}
	if v := partVolume(t, cs); v >= frameVol {
		t.Errorf("corner fillet removed no material (%.4g >= %.4g)", v, frameVol)
	}

	// 3. Central bore: cut a Ø47 mm circle through the frame.
	s = addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "23.5 mm"}, nil)
	bored := partVolume(t, cs)
	if h, r := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"}); !h {
		t.Fatalf("bore cut unhealthy: %s", r)
	}
	if v := partVolume(t, cs); v >= bored {
		t.Fatalf("bore cut removed no material (%.4g >= %.4g): the bore missed the frame", v, bored)
	}

	// 4. Central hub: join a Ø13.5 mm cylinder in the middle of the bore.
	s = addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "6.75 mm"}, nil)
	beforeHub := partVolume(t, cs)
	if h, r := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s, "profileIndex": 0, "distance": "15 mm", "operation": "join"}); !h {
		t.Fatalf("hub join unhealthy: %s", r)
	}
	if v := partVolume(t, cs); v <= beforeHub {
		t.Fatalf("hub join added no material (%.4g <= %.4g)", v, beforeHub)
	}

	// 5. One blade: a TWISTED radial fin LOFTED between two cross-sections on parallel work
	// planes — the root profile on XY (z=0) and a 20°-twisted tip profile on a work plane at
	// z=14 mm. This exercises multiple work planes, multiple sketches, and the loft tool.
	root := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": root, "kind": "rectangle", "points": [][]float64{{0.6, -0.08}, {2.35, 0.08}}}, nil)
	var tipWP struct {
		Index   int  `json:"index"`
		Healthy bool `json:"healthy"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "14 mm"}, &tipWP)
	if !tipWP.Healthy {
		t.Fatalf("blade tip work plane unhealthy")
	}
	var tipSk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": tipWP.Index}, &tipSk)
	// The same fin rotated 20° about the centre — a three-point rectangle is the rotated box.
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": tipSk.SketchIndex, "kind": "rectangle", "variant": "threePoint",
		"points": [][]float64{{0.591, 0.130}, {2.236, 0.729}, {2.182, 0.879}}}, nil)
	beforeBlade := partVolume(t, cs)
	blade, h, r := addNamedFeature(t, cs, "loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": root, "profileIndex": 0}, {"sketchIndex": tipSk.SketchIndex, "profileIndex": 0}},
		"operation": "join",
	})
	if !h {
		t.Fatalf("lofted blade unhealthy: %s", r)
	}
	if v := partVolume(t, cs); v <= beforeBlade {
		t.Fatalf("lofted blade added no material (%.4g <= %.4g)", v, beforeBlade)
	}

	// 6. Seven blades: circular-pattern the blade join (regression: a patterned join must merge
	// into one body and add material, not place separate solids).
	beforeFan := partVolume(t, cs)
	if h, r := applyFeature(t, cs, "patternCircular", map[string]any{
		"sourceFeatures": []string{blade}, "count": 7, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	}); !h {
		t.Fatalf("blade pattern unhealthy: %s", r)
	}
	if v := partVolume(t, cs); v <= beforeFan {
		t.Fatalf("blade pattern added no material (%.4g <= %.4g): the pattern did not replicate the join", v, beforeFan)
	}

	// 7. Four mounting holes at the corners (Ø3.4 mm), cut in one sketch.
	s = addSketchOn(t, cs)
	for _, c := range [][2]float64{{2, 2}, {-2, 2}, {-2, -2}, {2, -2}} {
		callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "circle", "points": [][]float64{{c[0], c[1]}}, "radius": "1.7 mm"}, nil)
	}
	beforeMounts := partVolume(t, cs)
	// Cut every profile of the mounting-hole sketch.
	for pi := 0; pi < 4; pi++ {
		applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s, "profileIndex": pi, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
	}
	if v := partVolume(t, cs); v >= beforeMounts {
		t.Errorf("mounting holes removed no material (%.4g >= %.4g)", v, beforeMounts)
	}

	// The whole fan is one solid body.
	var tree struct {
		Bodies   int `json:"bodies"`
		Features []struct {
			Kind string `json:"kind"`
		} `json:"features"`
	}
	callJSON(t, cs, "get_model_tree", nil, &tree)
	if tree.Bodies != 1 {
		t.Errorf("fan has %d bodies, want 1", tree.Bodies)
	}
	t.Logf("fan built: %d features, final volume %.3f cm³", len(tree.Features), partVolume(t, cs))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
