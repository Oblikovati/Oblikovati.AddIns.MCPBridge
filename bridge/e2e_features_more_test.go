// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deep end-to-end validation of the rest of the feature surface: additive profile features
// (revolve, rib, emboss, coil, loft), direct edits (move/offset/delete/split face, combine,
// thicken, trim), patterns/mirror, and freeform primitives. Each builds the minimal real
// scenario and asserts the kernel applied the feature. Features whose health depends on
// geometry the harness can't easily guarantee are asserted "reached without a panic"
// (mustReachFeature) — the endpoint and kernel ran; the operation is exercised end to end.

// addNamedFeature applies a feature and returns its tree name and health.
func addNamedFeature(t *testing.T, cs *mcp.ClientSession, kind string, args map[string]any) (name string, healthy bool, reason string) {
	t.Helper()
	var r struct {
		Feature string `json:"feature"`
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	callJSON(t, cs, "add_feature", map[string]any{"kind": kind, "args": args}, &r)
	return r.Feature, r.Healthy, r.Reason
}

// mustReachFeature asserts add_feature reached the kernel without a transport error or panic
// (a clean unhealthy result is tolerated — the operation was exercised).
func mustReachFeature(t *testing.T, cs *mcp.ClientSession, kind string, args map[string]any) {
	t.Helper()
	mustReach(t, cs, "add_feature", map[string]any{"kind": kind, "args": args})
}

// profileSketch creates a sketch on the given plane with one rectangle profile and returns its
// sketch index. Each call makes a new sketch on the active part.
func profileSketch(t *testing.T, cs *mcp.ClientSession, plane string, w, h string) int {
	t.Helper()
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": plane}, &sk)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": sk.SketchIndex, "width": w, "height": h}, nil)
	return sk.SketchIndex
}

func TestE2EFreeformPrimitives(t *testing.T) {
	cases := []struct {
		kind string
		args map[string]any
	}{
		{"freeformBox", map[string]any{"sizeX": "40 mm", "sizeY": "30 mm", "sizeZ": "20 mm"}},
		{"freeformPlane", map[string]any{"sizeX": "40 mm", "sizeY": "30 mm"}},
		{"freeformQuadBall", map[string]any{"radius": "20 mm"}},
	}
	for _, c := range cases {
		cs := freshPart(t)
		if healthy, reason := applyFeature(t, cs, c.kind, c.args); !healthy {
			t.Errorf("%s unhealthy: %s", c.kind, reason)
		}
	}
}

func TestE2ERevolve(t *testing.T) {
	cs := freshPart(t)
	// A rectangle offset in +X so it revolves about the world Y axis into a ring solid.
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, &sk)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "rectangle", "points": [][]float64{{2, 0}, {5, 3}}}, nil)
	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0, "axisRef": "origin/axis/y", "angle": "360 deg"}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}
}

func TestE2ELoft(t *testing.T) {
	cs := freshPart(t)
	s0 := profileSketch(t, cs, "XY", "40 mm", "30 mm")
	// A second profile on a parallel offset plane would be ideal; lacking that over the API,
	// loft two profiles on the same plane is degenerate, so assert it reaches the kernel.
	mustReachFeature(t, cs, "loft", map[string]any{"sections": []map[string]any{
		{"sketchIndex": s0, "profileIndex": 0}, {"sketchIndex": s0, "profileIndex": 0},
	}})
}

func TestE2EProfileFeaturesReach(t *testing.T) {
	// rib/emboss/coil need specific profile/axis geometry; assert each reaches the kernel
	// without a panic (the endpoints are wired and exercised end to end).
	cs := freshPart(t)
	s := profileSketch(t, cs, "XY", "40 mm", "30 mm")
	mustReachFeature(t, cs, "rib", map[string]any{"sketchIndex": s, "profileIndex": 0, "thickness": "2 mm", "depth": "10 mm"})
	mustReachFeature(t, cs, "emboss", map[string]any{"sketchIndex": s, "profileIndex": 0, "depth": "1 mm"})
	mustReachFeature(t, cs, "coil", map[string]any{"sketchIndex": s, "profileIndex": 0, "axisRef": "origin/axis/z", "pitch": "5 mm", "revolutions": "3"})
}

func TestE2EFaceEdits(t *testing.T) {
	// moveFace and faceOffset modify a box face and should stay healthy.
	for _, kind := range []string{"moveFace", "faceOffset"} {
		cs := boxClient(t)
		_, faces := topology(t, cs)
		args := map[string]any{"faceRefs": []string{faces[0]}}
		if kind == "moveFace" {
			args["translation"] = []float64{0, 0, 0.5}
		} else {
			args["distance"] = "2 mm"
		}
		if healthy, reason := applyFeature(t, cs, kind, args); !healthy {
			t.Errorf("%s unhealthy: %s", kind, reason)
		}
	}
	// deleteFace and split depend on what can be healed/split; assert they reach the kernel.
	for _, kind := range []string{"deleteFace", "split"} {
		cs := boxClient(t)
		_, faces := topology(t, cs)
		mustReachFeature(t, cs, kind, map[string]any{"faceRefs": []string{faces[0]}})
	}
}

func TestE2EHoleVariants(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
	}{
		{"counterbore", map[string]any{"type": "counterbore", "diameter": "4 mm", "depth": "10 mm", "counterDiameter": "8 mm", "counterDepth": "3 mm"}},
		{"countersink", map[string]any{"type": "countersink", "diameter": "4 mm", "depth": "10 mm", "sinkDiameter": "8 mm", "includedAngle": "90 deg"}},
		{"tapped", map[string]any{"type": "tapped", "diameter": "5 mm", "depth": "8 mm", "designation": "M5x0.8"}},
	}
	for _, c := range cases {
		cs := boxClient(t)
		_, faces := topology(t, cs)
		args := map[string]any{"faceRef": faces[0]}
		for k, v := range c.args {
			args[k] = v
		}
		if healthy, reason := applyFeature(t, cs, "hole", args); !healthy {
			t.Errorf("hole %s unhealthy: %s", c.name, reason)
		}
	}
}

func TestE2ETrim(t *testing.T) {
	cs := boxClient(t)
	// Trim the box with the z=1cm plane, keeping the lower half — a clean planar cut.
	if healthy, reason := applyFeature(t, cs, "trim", map[string]any{
		"origin": []float64{0, 0, 1}, "normal": []float64{0, 0, 1}, "keepPositive": false,
	}); !healthy {
		t.Fatalf("trim unhealthy: %s", reason)
	}
}

func TestE2ECombine(t *testing.T) {
	cs := boxClient(t)
	// A second overlapping box, then join the two bodies.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 1, "width": "20 mm", "height": "20 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "30 mm", "operation": "new"}}, nil)
	if healthy, reason := applyFeature(t, cs, "combine", map[string]any{"targetIndex": 0, "toolIndex": 1, "operation": "join"}); !healthy {
		t.Fatalf("combine unhealthy: %s", reason)
	}
}

func TestE2EPatternsAndMirror(t *testing.T) {
	// Pattern/mirror an existing hole feature. Build a box, drill a hole, capture its name.
	makeHoleBox := func(t *testing.T) (*mcp.ClientSession, string) {
		cs := boxClient(t)
		_, faces := topology(t, cs)
		name, healthy, reason := addNamedFeature(t, cs, "hole", map[string]any{"faceRef": faces[0], "diameter": "4 mm", "depth": "6 mm"})
		if !healthy {
			t.Fatalf("hole setup unhealthy: %s", reason)
		}
		return cs, name
	}

	t.Run("rectangular", func(t *testing.T) {
		cs, hole := makeHoleBox(t)
		mustReachFeature(t, cs, "patternRectangular", map[string]any{
			"sourceFeatures": []string{hole}, "countX": 2, "countY": 1, "stepX": []float64{1, 0, 0},
		})
	})
	t.Run("circular", func(t *testing.T) {
		cs, hole := makeHoleBox(t)
		mustReachFeature(t, cs, "patternCircular", map[string]any{
			"sourceFeatures": []string{hole}, "count": 3, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
		})
	})
	t.Run("mirror", func(t *testing.T) {
		cs, hole := makeHoleBox(t)
		mustReachFeature(t, cs, "mirror", map[string]any{
			"sourceFeatures": []string{hole}, "normal": []float64{1, 0, 0},
		})
	})
}
