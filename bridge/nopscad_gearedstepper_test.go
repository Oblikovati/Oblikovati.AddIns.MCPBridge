// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/app"
)

// TestNopGearedStepper28BYJ48 re-models the 28BYJ-48 geared tin-can stepper
// (NopSCADlib vitamins/geared_steppers.scad → geared_stepper.scad) as one native,
// fully-parametric Oblikovati part — the flagship "vitamin part" of the PT_camera
// example assembly (examples/PT_camera/PT_camera.scad), which mounts two of these
// steppers plus a camera into a pan/tilt rig. This is the parts-first half of porting
// that top-tier assembly: a separated, feature-based part the assembly later places as
// an occurrence and joints.
//
// Feature tree (every sketch FULLY CONSTRAINED to 0 DOF, sizes driven by named
// parameters lifted straight from the 28BYJ_48 property row):
//
//	can extrude(can_h) + rim fillet → shaft boss(boss_h, join) → output shaft(shaft_len,
//	join) → two tip flats (cut) giving the D-section → mounting-lug plate(lug_t, join) →
//	two screw holes(cut) → wire connector block(wire_h, join)
//
// The geometry is re-validated (one manifold/closed/oriented solid) after every feature,
// the final envelope is checked against the data-sheet dimensions, and a parametric edit
// (a wider can) rebuilds the whole stack — exercising the authoring path (sketch solver →
// feature recompute → B-rep) end to end, the surface the assembly tier depends on.
//
// 28BYJ_48 dimensions (mm), from the property row:
//
//	can Ø28 × 19, top rim r1, shaft offset 8 from can centre, boss Ø9 × 2×1.5,
//	shaft Ø5 × 10 with a 6 mm double-flat 3 mm across, screw pitch 35, lug Ø7 × 0.85,
//	screw hole Ø4.2, wire block 14.7 × 6 × 16.5 behind the can.
func TestNopGearedStepper28BYJ48(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}
	buildStepperPart(b)

	// --- Envelope: the part spans the can front (+6) to the wire block back (-25) in Y,
	// the lug ends (±21) in X, and the shaft tip (-10) to the can top (+19) in Z (mm). ---
	assertEnvelope(t, cs, [3][2]float64{{-2.1, 2.1}, {-2.5, 0.6}, {-1.0, 1.9}})
	// The Ø28×19 can dominates (a 32-gon prism ≈ 11.6 cm³); the boss, shaft, mounting lug and
	// wire block add ~1.3 more, so the whole part is ~12.9 cm³ — well inside its 4.2×3.1×2.9
	// envelope (≈38 cm³). A band around that catches a gross modelling/boolean error.
	if v := partVolume(t, cs); v < 11.0 || v > 15.0 {
		t.Errorf("stepper volume = %.4f cm^3, want ~12.9 (11–15 band)", v)
	}

	// --- Parametric: a wider can must rebuild the whole stack into one valid solid. ---
	callJSON(t, cs, "set_parameter", map[string]any{"name": "can_dia", "expression": "30 mm"}, nil)
	b.mustValid("reparam")
}

// rectOn draws a fully-connected rectangle in sketch si from two opposite corners and
// returns its four lines and corner points (bl, br, tr, tl). It generalises the
// sketch-0-only rectFull helper to any sketch index.
func rectOn(t *testing.T, cs *mcp.ClientSession, si int, pts [][]float64) rectComposite {
	t.Helper()
	var r struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": si, "kind": "rectangle", "points": pts}, &r)
	if len(r.EntityIDs) < 4 || len(r.PointIDs) < 4 {
		t.Fatalf("rectangle reply: lines=%v points=%v", r.EntityIDs, r.PointIDs)
	}
	return rectComposite{lines: r.EntityIDs, points: r.PointIDs}
}

// topRimEdgeKey returns the reference key of the body's highest circular edge — the can's
// top rim — so the rounding fillet targets it deterministically.
func topRimEdgeKey(t *testing.T, cs *mcp.ClientSession) string {
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
	if len(rk.Bodies) == 0 || len(rk.Bodies[0].Edges) == 0 {
		t.Fatal("get_reference_keys returned no edge topology for the can")
	}
	best, bestZ := "", math.Inf(-1)
	for _, e := range rk.Bodies[0].Edges {
		if len(e.Point) == 3 && e.Point[2] > bestZ {
			best, bestZ = e.Key, e.Point[2]
		}
	}
	if best == "" {
		t.Fatal("no edge carried a representative point; cannot pick the top rim")
	}
	return best
}

// assertEnvelope checks the active body's axis-aligned range box (cm) against expected
// [min,max] bounds per axis, with a 1 mm tolerance for fillet/round softening.
func assertEnvelope(t *testing.T, cs *mcp.ClientSession, want [3][2]float64) {
	t.Helper()
	var rb struct {
		Min []float64 `json:"min"`
		Max []float64 `json:"max"`
	}
	callJSON(t, cs, "body_range_box", map[string]any{"bodyIndex": 0}, &rb)
	if len(rb.Min) != 3 || len(rb.Max) != 3 {
		t.Fatalf("range box reply min=%v max=%v", rb.Min, rb.Max)
	}
	const tol = 0.1 // cm
	axes := []string{"X", "Y", "Z"}
	for a := 0; a < 3; a++ {
		if math.Abs(rb.Min[a]-want[a][0]) > tol {
			t.Errorf("%s min = %.3f cm, want %.3f (±%.2f)", axes[a], rb.Min[a], want[a][0], tol)
		}
		if math.Abs(rb.Max[a]-want[a][1]) > tol {
			t.Errorf("%s max = %.3f cm, want %.3f (±%.2f)", axes[a], rb.Max[a], want[a][1], tol)
		}
	}
}

// stepName labels the i-th replicate of a repeated feature step.
func stepName(base string, i int) string {
	if i == 0 {
		return base + "+Y"
	}
	return base + "-Y"
}
