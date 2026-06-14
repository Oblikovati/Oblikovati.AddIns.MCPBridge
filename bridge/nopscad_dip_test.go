// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// dipOctagonXY is the NopSCADlib DIP cross-section (the hull of the parting-line flash and the
// body) drawn in XY so it can be extruded along Z (the package length), CCW, cm. See
// kernel/brep TestNopDipCSG.
var dipOctagonXY = [][]float64{
	{-0.2675, 0.15}, {-0.3175, 0.0125}, {-0.3175, -0.0125}, {-0.2675, -0.15},
	{0.2675, -0.15}, {0.3175, -0.0125}, {0.3175, 0.0125}, {0.2675, 0.15},
}

// TestNopDip models the DIP IC package body the Inventor way, stacking the colliding booleans
// NopSCADlib uses: the chamfered hull octagon is extruded the package length, a central 4 mm
// slot is CUT through-all (disconnecting the bar into two separate wings), then a lower block
// and an upper block are JOIN-extruded back in along faces COPLANAR with the slot walls, re-
// welding the two wings into one watertight solid. This exercises a cut that splits a body into
// two lumps and a union that re-welds across coplanar faces — interactions simpler parts don't
// reach. (The pin-1 index notch is excluded — see kernel/brep TestBlindStraddleCurvedCutWatertight,
// the open spec for a planar-arrangement bug it triggers.)
func TestNopDip(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	const length = "8.89 mm" // size.x

	// 1. Chamfered hull octagon, extruded the package length.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "polyline", "closed": true, "points": dipOctagonXY,
	}, nil)
	if h, r := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": length, "operation": "new"}); !h {
		t.Fatalf("body extrude unhealthy: %s", r)
	}

	// 2. Central 4 mm slot, through-all ⇒ splits the bar into two wings.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": 1, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {0.2, 0.2}},
	}, nil)
	if h, r := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": length, "operation": "cut"}); !h {
		t.Fatalf("slot cut unhealthy: %s", r)
	}

	// 3. Re-weld: lower block (y −1.5..0.9 mm) + upper block (y 0..1.5 mm) refill the slot
	//    exactly, their x=±0.2 walls coplanar with the wing inner walls. The "center" variant's
	//    2nd point is an ABSOLUTE corner (half = |corner−center|), so corners are chosen to land
	//    the blocks on y[−0.15,0.09] and y[0,0.15] — together exactly the central strip.
	for i, rect := range [][2][2]float64{
		{{0, -0.03}, {0.2, 0.09}}, // lower spine, center (0,−0.03) corner (0.2,0.09) ⇒ y[−0.15,0.09]
		{{0, 0.075}, {0.2, 0.15}}, // upper block, center (0,0.075) corner (0.2,0.15) ⇒ y[0,0.15]
	} {
		si := 2 + i
		callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
		callJSON(t, cs, "add_sketch_entity", map[string]any{
			"sketchIndex": si, "kind": "rectangle", "variant": "center",
			"points": [][]float64{{rect[0][0], rect[0][1]}, {rect[1][0], rect[1][1]}},
		}, nil)
		if h, r := applyFeature(t, cs, "extrude",
			map[string]any{"sketchIndex": si, "profileIndex": 0, "distance": length, "operation": "join"}); !h {
			t.Fatalf("refill %d join unhealthy: %s", i, r)
		}
	}

	// One watertight solid — the two wings re-welded into the octagon prism.
	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 || !ops.Validate(bodies[0]).Valid || !ops.Validate(bodies[0]).Closed {
		t.Fatalf("dip body is not a single watertight solid (bodies=%d)", len(bodies))
	}

	// The body must stay within the octagon's footprint — a bbox check so a wrong refill (a
	// top-center gap balanced by a below-body protrusion) can't sneak past the volume check.
	bb := bodies[0].RangeBox()
	if bb.Min.Y < -0.151 || bb.Max.Y > 0.151 || bb.Min.X < -0.3176 || bb.Max.X > 0.3176 {
		t.Errorf("dip body bbox %v escapes the octagon footprint (refill misplaced)", bb)
	}

	// Slot + refill is volume-neutral ⇒ the octagon prism. Octagon area = 17.675 mm² (= 0.17675 cm²).
	want := 0.17675 * 0.889
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.02 {
		t.Errorf("dip body volume = %.6f cm^3, want ~%.6f (octagon prism)", got, want)
	}
}
