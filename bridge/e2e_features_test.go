// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This file validates every registered feature operation end to end over the bridge — the
// additive extrude AND the subtractive/dress-up family (fillet, chamfer, shell, draft, hole),
// which act on a real body's edges/faces by reference key. Each subtractive test builds a
// fresh box, reads its reference keys, applies the feature, and asserts the kernel produced
// healthy geometry (and, where it should, that the face count changed).

// boxClient returns a client whose active part holds one extruded 40×30×20 mm box.
func boxClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	cs := freshPart(t)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	var ext struct {
		Bodies int `json:"bodies"`
	}
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"},
	}, &ext)
	if ext.Bodies != 1 {
		t.Fatalf("box extrude produced %d bodies, want 1", ext.Bodies)
	}
	return cs
}

// topology reads the active body's edge and face reference keys (from get_reference_keys).
func topology(t *testing.T, cs *mcp.ClientSession) (edges, faces []string) {
	t.Helper()
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key string `json:"key"`
			} `json:"faces"`
			Edges []struct {
				Key string `json:"key"`
			} `json:"edges"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		t.Fatal("get_reference_keys returned no bodies")
	}
	for _, e := range rk.Bodies[0].Edges {
		edges = append(edges, e.Key)
	}
	for _, f := range rk.Bodies[0].Faces {
		faces = append(faces, f.Key)
	}
	// Stable order so a test picks the same edge/face run to run.
	sort.Strings(edges)
	sort.Strings(faces)
	return edges, faces
}

// applyFeature applies an add_feature call and returns its health (the result's "kind" is the
// model feature's own kind, e.g. "freeform"/"move-face", which need not equal the operation
// name, so it is not asserted here).
func applyFeature(t *testing.T, cs *mcp.ClientSession, kind string, args map[string]any) (healthy bool, reason string) {
	t.Helper()
	var r struct {
		Healthy bool   `json:"healthy"`
		Reason  string `json:"reason"`
	}
	callJSON(t, cs, "add_feature", map[string]any{"kind": kind, "args": args}, &r)
	return r.Healthy, r.Reason
}

// TestE2EFeatureRegistryCoverage is the drift guard: the registry must advertise exactly the
// feature kinds this suite deep-tests, so a new kind forces a new test (it fails until added).
func TestE2EFeatureRegistryCoverage(t *testing.T) {
	cs := freshPart(t)
	var kinds struct {
		Kinds []struct {
			Kind string `json:"kind"`
		} `json:"kinds"`
	}
	callJSON(t, cs, "list_feature_kinds", nil, &kinds)
	got := map[string]bool{}
	for _, k := range kinds.Kinds {
		got[k.Kind] = true
	}
	want := []string{
		"extrude", "revolve", "rib", "emboss", "coil", "loft",
		"fillet", "chamfer", "shell", "draft", "lip", "hole", "boss", "grill", "thread",
		"combine", "thicken", "trim", "directEdit", "moveFace", "faceOffset", "deleteFace",
		"simplify", "unwrap", "modelTolerance", "split",
		"replaceFace", "moveBody", "bendPart", "splitSolid", "coreCavity", "hull",
		"sweep", "patternRectangular", "patternCircular", "mirror", "patternSketchDriven",
		"boundaryPatch", "ruledSurface", "surfaceOffset", "extend", "midSurface", "stitch", "sculpt",
		"freeformBox", "freeformPlane", "freeformQuadBall", "mesh",
		// Sheet metal (M13-F02): exercised over the bridge by TestE2ESheetMetal{Style,Features}
		// (contour roll's centerline axis isn't expressible over the sketch wire, so it is
		// deep-tested in the source model/opregistry suites instead).
		"sheetMetalFace", "sheetMetalFlange", "sheetMetalHem", "sheetMetalBend", "sheetMetalFold",
		"sheetMetalCorner", "sheetMetalContourFlange", "sheetMetalLoftedFlange", "sheetMetalContourRoll",
		"sheetMetalCornerSeam",
		// Sheet metal (M13-F03): cut a profile through the part (through-all or to a depth).
		"sheetMetalCut",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("feature registry is missing %q (add_feature can't drive it)", w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("registry has %d kinds %v; update this suite to deep-test the new one(s)", len(got), kinds.Kinds)
	}
}

func TestE2EFillet(t *testing.T) {
	cs := boxClient(t)
	edges, _ := topology(t, cs)
	_, facesBefore := topology(t, cs)
	if healthy, reason := applyFeature(t, cs, "fillet", map[string]any{"edgeRefs": []string{edges[0]}, "radius": "3 mm"}); !healthy {
		t.Fatalf("fillet unhealthy: %s", reason)
	}
	if _, facesAfter := topology(t, cs); len(facesAfter) <= len(facesBefore) {
		t.Errorf("fillet did not add a face: before=%d after=%d", len(facesBefore), len(facesAfter))
	}
}

func TestE2EChamfer(t *testing.T) {
	cs := boxClient(t)
	edges, facesBefore := topology(t, cs)
	if healthy, reason := applyFeature(t, cs, "chamfer", map[string]any{"edgeRefs": []string{edges[0]}, "distance": "2 mm"}); !healthy {
		t.Fatalf("chamfer unhealthy: %s", reason)
	}
	if _, facesAfter := topology(t, cs); len(facesAfter) <= len(facesBefore) {
		t.Errorf("chamfer did not add a face: before=%d after=%d", len(facesBefore), len(facesAfter))
	}
}

func TestE2EShell(t *testing.T) {
	cs := boxClient(t)
	_, faces := topology(t, cs)
	if healthy, reason := applyFeature(t, cs, "shell", map[string]any{"faceRefs": []string{faces[0]}, "thickness": "2 mm"}); !healthy {
		t.Fatalf("shell unhealthy: %s", reason)
	}
	// Hollowing a box (one face removed) multiplies the face count (inner + outer walls).
	if _, faces2 := topology(t, cs); len(faces2) <= len(faces) {
		t.Errorf("shell did not hollow the body: faces %d -> %d", len(faces), len(faces2))
	}
}

func TestE2EHole(t *testing.T) {
	cs := boxClient(t)
	_, faces := topology(t, cs)
	if healthy, reason := applyFeature(t, cs, "hole", map[string]any{"faceRef": faces[0], "diameter": "5 mm", "depth": "8 mm"}); !healthy {
		t.Fatalf("hole unhealthy: %s", reason)
	}
	if _, faces2 := topology(t, cs); len(faces2) <= len(faces) {
		t.Errorf("hole did not add the cylindrical wall face: faces %d -> %d", len(faces), len(faces2))
	}
}

func TestE2EBendPart(t *testing.T) {
	cs := boxClient(t)
	// A bend line on its own sketch, crossing the box's bottom face (cm in sketch space).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": 1, "kind": "line", "points": [][]float64{{2, 0}, {2, 3}},
	}, nil)
	// Folding the block adds the bent wall faces; health is the success signal.
	if healthy, reason := applyFeature(t, cs, "bendPart", map[string]any{
		"sketchIndex": 1, "lineIndex": 0, "bendType": "radiusAndAngle", "radius": "5 mm", "angle": "90 deg",
	}); !healthy {
		t.Fatalf("bendPart unhealthy: %s", reason)
	}
}

func TestE2EDraft(t *testing.T) {
	cs := boxClient(t)
	_, faces := topology(t, cs)
	// Draft a face by a small angle; assert the kernel applied it (healthy) — the face count
	// is unchanged by a draft, so health is the success signal.
	if healthy, reason := applyFeature(t, cs, "draft", map[string]any{"faceRefs": []string{faces[0]}, "angle": "3 deg"}); !healthy {
		t.Fatalf("draft unhealthy: %s", reason)
	}
}

func TestE2ELip(t *testing.T) {
	cs := boxClient(t)
	edges, _ := topology(t, cs)
	// A raised lip bead along one edge adds the bead's walls; health is the success signal.
	if healthy, reason := applyFeature(t, cs, "lip", map[string]any{"edgeRefs": []string{edges[0]}, "width": "2 mm", "height": "2 mm"}); !healthy {
		t.Fatalf("lip unhealthy: %s", reason)
	}
}

func TestE2EGrill(t *testing.T) {
	cs := freshPart(t)
	// A 10×10×1 panel (sketch 0).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0, 0}, {10, 10}}}, nil)
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "1 cm", "operation": "new"}}, nil)
	// A vent boundary (6×6) bridged by two ribs (inner rectangles ⇒ holes) on sketch 1.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "rectangle", "points": [][]float64{{2, 2}, {8, 8}}}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "rectangle", "points": [][]float64{{3.75, 2.5}, {4.25, 7.5}}}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "rectangle", "points": [][]float64{{5.75, 2.5}, {6.25, 7.5}}}, nil)
	// Cutting the vent (bridged by the ribs) leaves one valid solid; health is the success signal.
	if healthy, reason := applyFeature(t, cs, "grill", map[string]any{"sketchIndex": 1, "boundaries": []int{0}}); !healthy {
		t.Fatalf("grill unhealthy: %s", reason)
	}
}

func TestE2ESimplify(t *testing.T) {
	cs := boxClient(t)
	// fillVoids on a solid box is a no-op heal that must keep it a valid solid; health signals success.
	if healthy, reason := applyFeature(t, cs, "simplify", map[string]any{"fillVoids": true}); !healthy {
		t.Fatalf("simplify unhealthy: %s", reason)
	}
}

func TestE2EUnwrap(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "20 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "30 mm", "operation": "new"}}, nil)
	// The cylindrical side face is the one at mid-height (the caps sit at z=0 and z=3 cm).
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	var side string
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[2] > 0.1 && f.Point[2] < 2.9 {
			side = f.Key
		}
	}
	if side == "" {
		t.Fatal("no cylindrical side face found to unwrap")
	}
	if healthy, reason := applyFeature(t, cs, "unwrap", map[string]any{"faceRef": side}); !healthy {
		t.Fatalf("unwrap unhealthy: %s", reason)
	}
}

func TestE2EModelTolerance(t *testing.T) {
	cs := boxClient(t)
	_, faces := topology(t, cs)
	// A GD&T feature-control frame + datum on a face: an annotation that changes no geometry,
	// so health is the success signal.
	if healthy, reason := applyFeature(t, cs, "modelTolerance", map[string]any{
		"frames": []map[string]any{{"geometry": faces[0], "characteristic": "flatness", "value": "0.05 mm", "datums": []string{"A"}}},
		"datums": []map[string]any{{"geometry": faces[0], "label": "A"}},
	}); !healthy {
		t.Fatalf("modelTolerance unhealthy: %s", reason)
	}
}
