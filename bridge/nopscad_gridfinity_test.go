// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/app"
)

// TestNopGridfinityBin re-models a 1×1×3 Gridfinity bin (NopSCADlib printed/gridfinity.scad,
// examples/Gridfinity) as a native parametric part. Its signature is the stacking foot: a
// chamfered pedestal modelled as two 45° TAPERED extrudes (the lower and upper chamfers, each a
// frustum widening a square as it rises) with a straight foot between, then the bin body, then a
// hollow cavity — five features stacked coplanar on successive work planes.
//
// This is the first port to exercise the draft/taper extrude and a tall coplanar stack, so it
// stresses the tapered-prism builder and the join boolean at each shelf. The rounded corners
// (corner_r/foot_r/chamfer_r) of the real bin are simplified to square corners for v1.
//
// Profile (mm): foot 35.6 → (0.8 lower chamfer) → 37.2 → (1.8 straight) → 37.2 → (2.15 upper
// chamfer) → 41.5, then the 41.5 body up to z = 3×7 = 21, hollowed to 1.6 mm walls.
func TestNopGridfinityBin(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	b := &partBuilder{t: t, s: s, cs: cs}

	for _, p := range [][2]string{
		{"w1", "35.6 mm"}, {"w2", "37.2 mm"}, {"bw", "41.5 mm"}, {"lower_ch", "0.8 mm"},
		{"foot_h", "1.8 mm"}, {"upper_ch", "2.15 mm"}, {"body_h", "16.25 mm"},
		{"cav_w", "38.3 mm"}, {"cav_h", "14.25 mm"},
	} {
		b.param(p[0], p[1])
	}

	// 1. Lower chamfer: a 35.6 square drafted 45° up 0.8 mm → 37.2 at the top.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-1.78, -1.78}, {1.78, 1.78}}, "w1", "w1")
	b.feat("1-lower-chamfer", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "lower_ch", "taper": "45 deg", "operation": "new"})

	// 2. Straight foot: 37.2 square, 1.8 mm, on the chamfer top (z=0.8).
	sFoot := b.sketchOn(b.workPlane("origin/plane/xy", "0.8 mm"))
	b.rectFC(sFoot, [][]float64{{-1.86, -1.86}, {1.86, 1.86}}, "w2", "w2")
	b.feat("2-foot", "extrude", map[string]any{"sketchIndex": sFoot, "profileIndex": 0, "distance": "foot_h", "operation": "join"})

	// 3. Upper chamfer: 37.2 drafted 45° up 2.15 mm → 41.5 at z=4.75.
	sUp := b.sketchOn(b.workPlane("origin/plane/xy", "2.6 mm"))
	b.rectFC(sUp, [][]float64{{-1.86, -1.86}, {1.86, 1.86}}, "w2", "w2")
	b.feat("3-upper-chamfer", "extrude", map[string]any{"sketchIndex": sUp, "profileIndex": 0, "distance": "upper_ch", "taper": "45 deg", "operation": "join"})

	// 4. Bin body: 41.5 square from z=4.75 up to z=21.
	sBody := b.sketchOn(b.workPlane("origin/plane/xy", "4.75 mm"))
	b.rectFC(sBody, [][]float64{{-2.075, -2.075}, {2.075, 2.075}}, "bw", "bw")
	b.feat("4-body", "extrude", map[string]any{"sketchIndex": sBody, "profileIndex": 0, "distance": "body_h", "operation": "join"})

	// 5. Hollow cavity: 38.3 square pocket from the top down, leaving 1.6 mm walls and a floor.
	sCav := b.sketchOn(b.workPlane("origin/plane/xy", "21 mm"))
	b.rectFC(sCav, [][]float64{{-1.915, -1.915}, {1.915, 1.915}}, "cav_w", "cav_w")
	b.feat("5-cavity", "extrude", map[string]any{"sketchIndex": sCav, "profileIndex": 0, "distance": "cav_h", "direction": "negative", "operation": "cut"})

	// Envelope: the foot base 35.6 sits at z=0, the body 41.5 reaches z=21 (mm).
	assertEnvelope(t, cs, [3][2]float64{{-2.075, 2.075}, {-2.075, 2.075}, {0, 2.1}})
	if v := partVolume(t, cs); v <= 0 {
		t.Errorf("gridfinity bin volume = %.4f, want > 0", v)
	}

	// Parametric: a taller bin must rebuild the stack into one valid solid.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "body_h", "expression": "23.25 mm"}, nil)
	b.mustValid("reparam")
}
