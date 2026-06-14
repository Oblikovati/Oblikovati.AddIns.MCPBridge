// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestE2EDirectEditScale deep-tests the directEdit feature kind over the bridge: scaling the box
// body by a uniform factor multiplies its volume by the cube of that factor. (Drives the consolidated
// direct-edit op via add_feature, the path the registry-coverage guard pins.)
func TestE2EDirectEditScale(t *testing.T) {
	cs := boxClient(t) // 40×30×20 mm = 24 cm³
	before := partVolume(t, cs)
	if before <= 0 {
		t.Fatalf("box volume = %g, want > 0", before)
	}
	if healthy, reason := applyFeature(t, cs, "directEdit", map[string]any{"operation": "scale", "scale": 2.0}); !healthy {
		t.Fatalf("directEdit scale unhealthy: %s", reason)
	}
	after := partVolume(t, cs)
	if ratio := after / before; math.Abs(ratio-8) > 0.02 {
		t.Errorf("directEdit scale 2.0: volume %g cm³ from %g (ratio %g), want 8× (192 from 24)", after, before, ratio)
	}
}

// tetrahedronSTL is a minimal valid ASCII STL — a four-facet tetrahedron — used to drive the mesh
// feature without an external fixture file.
const tetrahedronSTL = `solid tetra
facet normal 0 0 -1
outer loop
vertex 0 0 0
vertex 0 10 0
vertex 10 0 0
endloop
endfacet
facet normal 0 -1 0
outer loop
vertex 0 0 0
vertex 10 0 0
vertex 0 0 10
endloop
endfacet
facet normal -1 0 0
outer loop
vertex 0 0 0
vertex 0 0 10
vertex 0 10 0
endloop
endfacet
facet normal 1 1 1
outer loop
vertex 10 0 0
vertex 0 10 0
vertex 0 0 10
endloop
endfacet
endsolid tetra
`

// TestE2EMeshReference deep-tests the mesh feature kind over the bridge: placing an ASCII STL adds a
// mesh-reference feature (selectable facet geometry that passes the solid through). Drives add_feature
// with a host-local STL path, the path the registry-coverage guard pins.
func TestE2EMeshReference(t *testing.T) {
	cs := freshPart(t)
	stl := filepath.Join(t.TempDir(), "tetra.stl")
	if err := os.WriteFile(stl, []byte(tetrahedronSTL), 0o644); err != nil {
		t.Fatalf("write STL: %v", err)
	}
	if healthy, reason := applyFeature(t, cs, "mesh", map[string]any{"path": stl}); !healthy {
		t.Fatalf("mesh place unhealthy: %s", reason)
	}
	var tree struct {
		Features []struct {
			Kind string `json:"kind"`
		} `json:"features"`
	}
	callJSON(t, cs, "get_model_tree", nil, &tree)
	found := false
	for _, f := range tree.Features {
		if f.Kind == "mesh" {
			found = true
		}
	}
	if !found {
		t.Errorf("placed mesh feature not in model tree: %+v", tree.Features)
	}
}
