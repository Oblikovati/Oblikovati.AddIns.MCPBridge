// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopBoxWithBosses re-models the full NopSCADlib printed box base (printed/box.scad box_base) as
// one feature-based part with SIX stacked features, going past the plain tray (TestNopBoxTrayParametric
// stops at the shell) to the corner screw bosses and their holes. Every sketch is FULLY CONSTRAINED
// (0 DOF) and driven by named parameters (the Inventor way), and a final set_parameter edit rebuilds
// the whole stack — so this exercises both feature interaction AND parametric recompute through the
// MCP API and the geometry kernel:
//
//	block extrude(H) → shell(wall, top open) → corner boss(bossD, join) → rectangular pattern (×4) →
//	screw hole(holeD, through) → pattern of the hole (×4)
//
// The interaction under test: a boss must weld to the HOLLOW shelled wall, the pattern must replicate
// that join four times without disconnecting, and the through-holes must cut the merged result. The
// body is re-validated (one manifold/closed/oriented solid) after every feature.
func TestNopBoxWithBosses(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	for _, p := range [][2]string{
		{"W", "40 mm"}, {"D", "30 mm"}, {"H", "20 mm"}, {"wall", "2 mm"},
		{"bossD", "8 mm"}, {"bossH", "18 mm"}, {"holeD", "3 mm"},
	} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	con := func(si int, kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents}, nil)
	}
	dim := func(si int, kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	solved := func(si int) {
		t.Helper()
		var sv struct {
			DOF int `json:"dof"`
		}
		callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": si}, &sv)
		if sv.DOF != 0 {
			t.Fatalf("sketch %d not fully constrained: dof=%d, want 0", si, sv.DOF)
		}
	}
	mustValid := func(step string) {
		t.Helper()
		part, err := modelaccess.ActivePart(s)
		if err != nil {
			t.Fatalf("%s: active part: %v", step, err)
		}
		bodies := part.SurfaceBodies().All()
		if len(bodies) != 1 {
			t.Fatalf("%s: want 1 body, got %d (a join/pattern disconnected the part)", step, len(bodies))
		}
		if r := ops.Validate(bodies[0]); !r.Valid {
			t.Fatalf("%s: INVALID (manifold=%v closed=%v orient=%v): %v",
				step, r.Manifold, r.Closed, r.OrientationOK, capIssues(r.Issues))
		}
	}
	feat := func(step, kind string, args map[string]any) string {
		t.Helper()
		if h, reason := applyFeature(t, cs, kind, args); !h {
			t.Fatalf("%s: feature %q unhealthy: %s", step, kind, reason)
		}
		mustValid(step)
		return lastFeatureName(t, cs)
	}

	// 1. W×D plate, corner at the origin, fully constrained to the W,D parameters.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	rect := rectFull(t, cs, [][]float64{{0, 0}, {4, 3}})
	bl, br, tr, tl := rect.points[0], rect.points[1], rect.points[2], rect.points[3]
	con(0, "ground", bl)
	con(0, "horizontal", bl, br)
	con(0, "horizontal", tl, tr)
	con(0, "vertical", bl, tl)
	con(0, "vertical", br, tr)
	dim(0, "distance", "W", bl, br)
	dim(0, "distance", "D", bl, tl)
	solved(0)
	feat("1-block", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "H", "operation": "new"})

	// 2. Hollow to a wall-thick tray, top face removed.
	feat("2-shell", "shell", map[string]any{"faceRefs": []string{topFaceKey(t, cs)}, "thickness": "wall"})

	// 3. A corner screw boss (Ø bossD), center grounded at (8,8) mm, diameter parametric; join.
	sBoss := addSketchOn(t, cs)
	boss := idsOf(t, cs, map[string]any{"sketchIndex": sBoss, "kind": "circle", "points": [][]float64{{0.8, 0.8}}, "radius": "0.4 cm"})
	con(sBoss, "ground", boss[1])
	dim(sBoss, "diameter", "bossD", boss[0])
	solved(sBoss)
	bossName := feat("3-boss", "extrude", map[string]any{"sketchIndex": sBoss, "profileIndex": 0, "distance": "bossH", "operation": "join"})

	// 4. Replicate the boss to all four corners (14 mm pitch).
	feat("4-boss-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{bossName}, "countX": 2, "countY": 2,
		"stepX": []float64{2.4, 0, 0}, "stepY": []float64{0, 1.4, 0},
	})

	// 5. A Ø holeD screw hole through one boss (center grounded on the boss axis), then 6. patterned.
	sHole := addSketchOn(t, cs)
	hole := idsOf(t, cs, map[string]any{"sketchIndex": sHole, "kind": "circle", "points": [][]float64{{0.8, 0.8}}, "radius": "0.15 cm"})
	con(sHole, "ground", hole[1])
	dim(sHole, "diameter", "holeD", hole[0])
	solved(sHole)
	holeName := feat("5-hole", "extrude", map[string]any{"sketchIndex": sHole, "profileIndex": 0, "extent": "through-all", "operation": "cut"})
	feat("6-hole-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{holeName}, "countX": 2, "countY": 2,
		"stepX": []float64{2.4, 0, 0}, "stepY": []float64{0, 1.4, 0},
	})

	// A hollow tray + four bored bosses is a fraction of the W×D×H envelope.
	if got := partVolume(t, cs); got <= 0 || got > 4.0*3.0*2.0 {
		t.Errorf("box-with-bosses volume = %.4f cm^3, out of range (0, 24)", got)
	}

	// Parametric: fatter bosses must rebuild the whole stack (join → pattern → bore → pattern) valid.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "bossD", "expression": "10 mm"}, nil)
	mustValid("7-reparam")
}
