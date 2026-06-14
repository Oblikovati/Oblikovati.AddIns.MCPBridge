// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// End-to-end validation of the advanced + surfacing feature operations: sweep, moveBody,
// replaceFace, coreCavity, splitSolid, sketch-driven pattern, and the surface family
// (boundaryPatch, ruledSurface, surfaceOffset, extend, midSurface, stitch, sculpt). Those
// with deterministic geometry assert healthy; the rest (which need surface bodies or specific
// tooling setup the harness can't cheaply guarantee) assert "reached without a panic" — the
// resolver and kernel path ran end to end.

func TestE2EMoveBody(t *testing.T) {
	cs := boxClient(t)
	if healthy, reason := applyFeature(t, cs, "moveBody", map[string]any{"bodyIndex": 0, "translation": []float64{1, 0, 0}}); !healthy {
		t.Fatalf("moveBody unhealthy: %s", reason)
	}
}

func TestE2EBoundaryPatch(t *testing.T) {
	cs := freshPart(t)
	s := profileSketch(t, cs, "XY", "40 mm", "30 mm")
	if healthy, reason := applyFeature(t, cs, "boundaryPatch", map[string]any{"sketchIndex": s, "profileIndex": 0}); !healthy {
		t.Fatalf("boundaryPatch unhealthy: %s", reason)
	}
}

func TestE2ESweepReaches(t *testing.T) {
	cs := freshPart(t)
	// Profile on XY, an open path line on XZ to sweep along.
	profile := profileSketch(t, cs, "XY", "8 mm", "8 mm")
	var pathSk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XZ"}, &pathSk)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": pathSk.SketchIndex, "kind": "line", "points": [][]float64{{0, 0}, {0, 20}}}, nil)
	mustReachFeature(t, cs, "sweep", map[string]any{
		"sketchIndex": profile, "profileIndex": 0, "pathSketchIndex": pathSk.SketchIndex, "pathIndex": 0,
	})
}

func TestE2ESplitSolidReaches(t *testing.T) {
	cs := boxClient(t)
	var planes struct {
		Planes []struct {
			Ref string `json:"ref"`
		} `json:"planes"`
	}
	callJSON(t, cs, "list_work_planes", nil, &planes)
	// An offset work plane through the box, then split along it.
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{planes.Planes[0].Ref}, "offset": "1 cm"}, nil)
	mustReachFeature(t, cs, "splitSolid", map[string]any{"workPlaneIndex": 3, "keep": "both"})
}

func TestE2EReplaceFaceReaches(t *testing.T) {
	cs := boxClient(t)
	_, faces := topology(t, cs)
	mustReachFeature(t, cs, "replaceFace", map[string]any{"faceRefs": []string{faces[0]}, "targetRef": faces[1]})
}

func TestE2ECoreCavityReaches(t *testing.T) {
	cs := boxClient(t)
	mustReachFeature(t, cs, "coreCavity", map[string]any{"axis": "z", "position": "10 mm", "shrinkage": 0.02})
}

func TestE2ESketchDrivenPatternReaches(t *testing.T) {
	cs := boxClient(t)
	_, faces := topology(t, cs)
	hole, healthy, reason := addNamedFeature(t, cs, "hole", map[string]any{"faceRef": faces[0], "diameter": "4 mm", "depth": "6 mm"})
	if !healthy {
		t.Fatalf("hole setup unhealthy: %s", reason)
	}
	mustReachFeature(t, cs, "patternSketchDriven", map[string]any{
		"sourceFeatures": []string{hole}, "points": [][]float64{{1, 1, 2}, {-1, -1, 2}},
	})
}

func TestE2ESurfaceFeaturesReach(t *testing.T) {
	// These act on surface bodies / specific configurations; assert each reaches the kernel
	// without a panic (the resolver and feature path run end to end).
	t.Run("ruledSurface", func(t *testing.T) {
		cs := freshPart(t)
		s := profileSketch(t, cs, "XY", "40 mm", "30 mm")
		mustReachFeature(t, cs, "ruledSurface", map[string]any{"sketchIndex": s, "profileIndex": 0, "type": "normal", "distance": "10 mm"})
	})
	t.Run("surfaceOffset", func(t *testing.T) {
		cs := freshPart(t)
		profileSketch(t, cs, "XY", "40 mm", "30 mm")
		mustReachFeature(t, cs, "surfaceOffset", map[string]any{"distance": "2 mm"})
	})
	t.Run("midSurface", func(t *testing.T) {
		cs := boxClient(t)
		mustReachFeature(t, cs, "midSurface", map[string]any{"maxThickness": "30 mm"})
	})
	t.Run("stitch", func(t *testing.T) {
		cs := freshPart(t)
		mustReachFeature(t, cs, "stitch", map[string]any{"tolerance": "0.1 mm"})
	})
	t.Run("sculpt", func(t *testing.T) {
		cs := boxClient(t)
		mustReachFeature(t, cs, "sculpt", map[string]any{"operation": "new", "tolerance": "0.1 mm"})
	})
	t.Run("extend", func(t *testing.T) {
		cs := boxClient(t)
		edges, _ := topology(t, cs)
		mustReachFeature(t, cs, "extend", map[string]any{"edgeRef": edges[0], "distance": "5 mm"})
	})
}
