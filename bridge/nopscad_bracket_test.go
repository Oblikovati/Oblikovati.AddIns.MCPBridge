// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopBracketDraftRib models an angle bracket as one feature-based part of SIX stacked features —
// every sketch fully constrained to 0 DOF and parameter-driven — exercising the molding features DRAFT
// and RIB and how they interact with the rest of the stack:
//
//	base plate(bt) → upright wall(wt, join → L) → DRAFT the wall outer face(draftA) → RIB gusset in the
//	corner(ribT, join) → mounting hole(holeD) in the base → rectangular pattern of the hole (×2)
//
// Re-validated as one manifold/closed/oriented solid after every feature.
func TestNopBracketDraftRib(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	for _, p := range [][2]string{
		{"bw", "40 mm"}, {"bd", "30 mm"}, {"bt", "4 mm"},
		{"wh", "30 mm"}, {"wt", "4 mm"}, {"draftA", "5 deg"}, {"ribT", "4 mm"}, {"holeD", "4 mm"},
	} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	con := func(si int, kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents}, nil)
	}
	dim := func(si int, kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	axisRect := func(si int, pts [][]float64, wExpr, hExpr string) []uint64 {
		t.Helper()
		var r struct {
			PointIDs []uint64 `json:"pointIds"`
		}
		callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": si, "kind": "rectangle", "points": pts}, &r)
		bl, br, tr, tl := r.PointIDs[0], r.PointIDs[1], r.PointIDs[2], r.PointIDs[3]
		con(si, "ground", bl)
		con(si, "horizontal", bl, br)
		con(si, "horizontal", tl, tr)
		con(si, "vertical", bl, tl)
		con(si, "vertical", br, tr)
		dim(si, "distance", wExpr, bl, br)
		dim(si, "distance", hExpr, bl, tl)
		return r.PointIDs
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
			t.Fatalf("%s: want 1 body, got %d", step, len(bodies))
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
	faceAt := func(want [3]float64, tol float64) string {
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
		for _, f := range rk.Bodies[0].Faces {
			if len(f.Point) == 3 && math.Abs(f.Point[0]-want[0]) < tol && math.Abs(f.Point[1]-want[1]) < tol && math.Abs(f.Point[2]-want[2]) < tol {
				return f.Key
			}
		}
		t.Fatalf("no face near %v", want)
		return ""
	}

	// 1. bw×bd base plate, corner at the origin.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	axisRect(0, [][]float64{{0, 0}, {4, 3}}, "bw", "bd")
	solved(0)
	feat("1-base", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "bt", "operation": "new"})

	// 2. Upright wall on the y=0 edge (XZ sketch), joined into an L.
	sW := addSketchOnPlane(t, cs, "XZ")
	axisRect(sW, [][]float64{{0, 0}, {4, 3}}, "bw", "wh")
	solved(sW)
	feat("2-wall", "extrude", map[string]any{"sketchIndex": sW, "profileIndex": 0, "distance": "wt", "operation": "join"})

	// 3. Draft the wall's outer face (y=0) so the wall tapers.
	feat("3-draft", "draft", map[string]any{"faceRefs": []string{faceAt([3]float64{2, 0, 1.7}, 0.4)}, "angle": "draftA"})

	// 4. A corner gusset RIB (open diagonal in the inner corner on the YZ plane), joined.
	sR := addSketchOnPlane(t, cs, "YZ")
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sR, "kind": "line", "points": [][]float64{{0.4, 2.0}, {2.0, 0.4}}}, nil)
	feat("4-rib", "rib", map[string]any{"sketchIndex": sR, "profileIndex": 0, "thickness": "ribT", "depth": "20 mm", "operation": "join"})

	// 5. A mounting hole through the base, then 6. pattern it.
	sH := addSketchOn(t, cs)
	hole := idsOf(t, cs, map[string]any{"sketchIndex": sH, "kind": "circle", "points": [][]float64{{2.5, 1.5}}, "radius": "0.2 cm"})
	con(sH, "ground", hole[1])
	dim(sH, "diameter", "holeD", hole[0])
	solved(sH)
	holeName := feat("5-hole", "extrude", map[string]any{"sketchIndex": sH, "profileIndex": 0, "extent": "through-all", "operation": "cut"})
	feat("6-hole-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{holeName}, "countX": 2, "countY": 1, "stepX": []float64{1, 0, 0},
	})

	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("bracket volume = %.4f cm^3, want > 0", got)
	}
}
