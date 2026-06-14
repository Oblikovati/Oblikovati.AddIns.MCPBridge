// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Feature-interaction tests: multi-feature workflows where later features reference geometry
// created by earlier ones — in particular a work plane built on a feature-created face, a
// sketch on that work plane, and features driven from it. This is the integration the single
// -feature tests miss (a "car wheel": a disc, bolt holes on a work plane above it, a circular
// pattern, and a fillet).

// facePt is a body face reference key with its representative point.
type facePt struct {
	key   string
	point [3]float64
}

// topFace reads the active body's faces and returns the one whose representative point has the
// greatest Z (the disc's top planar face) — the surface to build a work plane on.
func topFace(t *testing.T, cs *mcp.ClientSession) facePt {
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
		t.Fatal("no bodies for reference keys")
	}
	var best facePt
	found := false
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) != 3 {
			continue
		}
		if !found || f.Point[2] > best.point[2] {
			best = facePt{f.Key, [3]float64{f.Point[0], f.Point[1], f.Point[2]}}
			found = true
		}
	}
	if !found {
		t.Fatal("no planar face found on the body")
	}
	return best
}

// partVolume reads the active part's volume (cm³) via get_physical_properties.
func partVolume(t *testing.T, cs *mcp.ClientSession) float64 {
	t.Helper()
	var pp struct {
		Volume float64 `json:"volume"`
	}
	callJSON(t, cs, "get_physical_properties", nil, &pp)
	return pp.Volume
}

// TestE2EWheelWorkflow drives the full interaction chain and asserts the geometry built on a
// feature-created face is healthy at each dependent step.
func TestE2EWheelWorkflow(t *testing.T) {
	cs := freshPart(t)

	// 1. Disc blank: a Ø60 mm circle extruded 10 mm thick.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "30 mm"}, nil)
	var disc struct {
		Bodies  int  `json:"bodies"`
		Healthy bool `json:"healthy"`
	}
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "10 mm", "operation": "new"}}, &disc)
	if disc.Bodies != 1 || !disc.Healthy {
		t.Fatalf("disc blank: bodies=%d healthy=%v", disc.Bodies, disc.Healthy)
	}
	discVolume := partVolume(t, cs)

	// 2. A work plane on the disc's TOP face (a face the extrude feature created), offset 5 mm.
	top := topFace(t, cs)
	var wp struct {
		Index   int  `json:"index"`
		Healthy bool `json:"healthy"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{top.key}, "offset": "5 mm"}, &wp)
	if !wp.Healthy {
		t.Fatalf("work plane on the feature-created face is unhealthy (index %d)", wp.Index)
	}

	// 3. A sketch ON that work plane — the capability under test.
	var sk struct {
		SketchIndex int    `json:"sketchIndex"`
		Plane       string `json:"plane"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	if sk.SketchIndex < 1 {
		t.Fatalf("sketch on work plane got index %d (plane %q), want a new sketch", sk.SketchIndex, sk.Plane)
	}

	// 4. A bolt-hole circle on the work plane, 20 mm out from the centre, cut down through the disc.
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "circle", "points": [][]float64{{2, 0}}, "radius": "3 mm"}, nil)
	boltName, healthy, reason := addNamedFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": "20 mm", "operation": "cut", "direction": "negative",
	})
	if !healthy {
		t.Fatalf("bolt-hole cut from the work-plane sketch is unhealthy: %s", reason)
	}
	// The cut must actually remove material — a hole placed off the body is a healthy no-op, so
	// volume is what proves the sketch on the work plane landed on the disc (regression for the
	// work-plane-on-face origin sitting at a rim vertex).
	oneHole := partVolume(t, cs)
	if oneHole >= discVolume {
		t.Fatalf("bolt cut removed no material (volume %.4g >= disc %.4g): the hole missed the body", oneHole, discVolume)
	}

	// 5. Circular-pattern the bolt hole around the wheel centre. Patterning a cut must keep one
	// body with more holes (regression for pattern-of-cut splitting into N solids), so the
	// volume drops further.
	if h, r := applyFeature(t, cs, "patternCircular", map[string]any{
		"sourceFeatures": []string{boltName}, "count": 5, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	}); !h {
		t.Fatalf("circular pattern of the bolt cut is unhealthy: %s", r)
	}
	if fiveHoles := partVolume(t, cs); fiveHoles >= oneHole {
		t.Fatalf("circular pattern removed no extra material (volume %.4g >= one-hole %.4g): the pattern did not replicate the cut", fiveHoles, oneHole)
	}

	// 6. Fillet a disc edge — a dress-up on the still-one body after the pattern. (Regression
	// for "edge reference lost": the broken pattern used to leave inconsistent topology that
	// made the edge key unresolvable; the fix keeps one coherent body, so the fillet resolves.)
	edges, _ := topology(t, cs)
	if h, r := applyFeature(t, cs, "fillet", map[string]any{"edgeRefs": []string{edges[0]}, "radius": "1 mm"}); !h {
		t.Fatalf("fillet on the wheel is unhealthy (edge reference lost?): %s", r)
	}

	// 7. The model tree records the whole interacting program on ONE body.
	var tree struct {
		Bodies   int `json:"bodies"`
		Features []struct {
			Kind string `json:"kind"`
		} `json:"features"`
	}
	callJSON(t, cs, "get_model_tree", nil, &tree)
	if tree.Bodies != 1 {
		t.Errorf("wheel has %d bodies, want 1 (the patterned cut must not split it)", tree.Bodies)
	}
	if len(tree.Features) < 4 {
		t.Errorf("wheel program has %d features, want >= 4 (disc, bolt cut, pattern, fillet)", len(tree.Features))
	}
}

// TestE2EWorkPlaneOnFaceThenSketch isolates the core capability: a work plane on a
// feature-created face, then a sketch on it, then a feature from that sketch — the chain that
// was impossible before create_sketch accepted a work plane.
func TestE2EWorkPlaneOnFaceThenSketch(t *testing.T) {
	cs := boxClient(t) // a 40x30x20 box from an extrude
	top := topFace(t, cs)

	var wp struct {
		Index   int  `json:"index"`
		Healthy bool `json:"healthy"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{top.key}, "offset": "10 mm"}, &wp)
	if !wp.Healthy {
		t.Fatalf("work plane on the box's top face is unhealthy")
	}

	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": sk.SketchIndex, "width": "10 mm", "height": "10 mm"}, nil)

	// A boss extruded off the work plane down onto the box (join) — a feature on a feature face.
	if h, r := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": "12 mm", "operation": "join", "direction": "negative",
	}); !h {
		t.Fatalf("extrude from the work-plane sketch is unhealthy: %s", r)
	}
}
