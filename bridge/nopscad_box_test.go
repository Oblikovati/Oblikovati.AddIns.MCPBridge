// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopBoxTrayParametric models a printed box base (NopSCADlib printed/box.scad) the
// Inventor way: a fully-constrained W×D rectangle extruded H into a block, then the SHELL
// feature hollows it to a wall thickness with the top face removed — a tray. The top face is
// picked from the body's reference keys by its representative point's Z (deterministic, no
// viewport pick). A parameter edit (raise the box) must rebuild the shell and grow the volume.
//
// Reference: NopSCADlib/printed/box.scad (box_base).
func TestNopBoxTrayParametric(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "W", "expression": "40 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "D", "expression": "30 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "H", "expression": "20 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "wall", "expression": "2 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Base rectangle, corner at the origin, fully constrained to W×D.
	rect := rectFull(t, cs, [][]float64{{0, 0}, {4, 3}})
	bl, br, tr, tl := rect.points[0], rect.points[1], rect.points[2], rect.points[3]
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	con("ground", bl)
	con("horizontal", bl, br)
	con("horizontal", tl, tr)
	con("vertical", bl, tl)
	con("vertical", br, tr)
	dim := func(expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": ents, "expression": expr}, nil)
	}
	dim("W", bl, br)
	dim("D", bl, tl)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("box base sketch not fully constrained: dof=%d, want 0", solve.DOF)
	}

	if healthy, reason := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "H", "operation": "new"}); !healthy {
		t.Fatalf("extrude unhealthy: %s", reason)
	}

	if healthy, reason := applyFeature(t, cs, "shell",
		map[string]any{"faceRefs": []string{topFaceKey(t, cs)}, "thickness": "wall"}); !healthy {
		t.Fatalf("shell unhealthy: %s", reason)
	}

	// Tray volume = outer block − inner void (open top): W·D·H − (W−2t)(D−2t)(H−t), in cm^3.
	wantVol := func(wMM, dMM, hMM, tMM float64) float64 {
		w, d, h, tt := wMM/10, dMM/10, hMM/10, tMM/10
		return w*d*h - (w-2*tt)*(d-2*tt)*(h-tt)
	}
	if got, w := partVolume(t, cs), wantVol(40, 30, 20, 2); math.Abs(got-w)/w > 0.02 {
		t.Errorf("box-tray volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Parametric: a taller box adds wall material on the four sides; the shell rebuilds.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "H", "expression": "30 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(40, 30, 30, 2); math.Abs(got-w)/w > 0.02 {
		t.Errorf("raised box-tray volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// topFaceKey returns the reference key of the active body's top (+Z) face — the one whose
// representative point has the greatest Z. Deterministic, so the shell removes the open top.
func topFaceKey(t *testing.T, cs *mcp.ClientSession) string {
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
	if len(rk.Bodies) == 0 || len(rk.Bodies[0].Faces) == 0 {
		t.Fatal("get_reference_keys returned no faces")
	}
	best, bestZ := "", math.Inf(-1)
	for _, f := range rk.Bodies[0].Faces {
		if len(f.Point) == 3 && f.Point[2] > bestZ {
			best, bestZ = f.Key, f.Point[2]
		}
	}
	if best == "" {
		t.Fatal("no face carried a representative point")
	}
	return best
}
