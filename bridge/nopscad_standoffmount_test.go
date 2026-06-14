// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopStandoffMount models a PCB standoff mount as one feature-based part of FIVE stacked features
// — every sketch fully constrained to 0 DOF and parameter-driven — exercising a FILLET, the counterbore
// HOLE feature, and a group PATTERN:
//
//	plate extrude(pt) → FILLET the plate's four corners(fr) → SQUARE post(postS, join) →
//	counterbore HOLE(boreD/cbD) into the post top → rectangular pattern of [post, hole] (×4)
//
// The fillet is on the plate's standalone vertical corner edges (the clean case). It deliberately does
// NOT chamfer/fillet the post itself: a round post rim is a faceted prism (Oblikovati/Oblikovati#127),
// and fillet/chamfer of the post's edges fails at the post→plate JOIN runout (an "inconsistent
// orientation" / non-manifold result) — a separate fragility noted on that issue. The interaction under
// test here: the counterbore hole references the join's top face, and the pattern replays the post+hole
// group at four corners, re-resolving the hole's face reference on each copy. Re-validated as one
// manifold/closed/oriented solid after every feature.
func TestNopStandoffMount(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	for _, p := range [][2]string{
		{"pw", "40 mm"}, {"pd", "40 mm"}, {"pt", "3 mm"}, {"fr", "2 mm"},
		{"postS", "10 mm"}, {"postH", "13 mm"}, {"boreD", "3 mm"}, {"cbD", "6 mm"},
	} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	con := func(si int, kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents}, nil)
	}
	dim := func(si int, kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	axisRect := func(si int, pts [][]float64, wExpr, hExpr string) {
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

	// 1. pw×pd plate, corner at the origin.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	rect := rectFull(t, cs, [][]float64{{0, 0}, {4, 4}})
	bl, br, tr, tl := rect.points[0], rect.points[1], rect.points[2], rect.points[3]
	con(0, "ground", bl)
	con(0, "horizontal", bl, br)
	con(0, "horizontal", tl, tr)
	con(0, "vertical", bl, tl)
	con(0, "vertical", br, tr)
	dim(0, "distance", "pw", bl, br)
	dim(0, "distance", "pd", bl, tl)
	solved(0)
	feat("1-plate", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "pt", "operation": "new"})

	// 2. Round the plate's four vertical corner edges (standalone block edges — the clean fillet case).
	var corners []string
	for _, e := range bodyEdges(t, cs) {
		if (e.point[0] < 0.1 || e.point[0] > 3.9) && (e.point[1] < 0.1 || e.point[1] > 3.9) && e.point[2] > 0.05 && e.point[2] < 0.25 {
			corners = append(corners, e.key)
		}
	}
	if len(corners) < 4 {
		t.Fatalf("want 4 plate corner edges, found %d", len(corners))
	}
	feat("2-fillet", "fillet", map[string]any{"edgeRefs": corners[:4], "radius": "fr"})

	// 3. A square Ø postS standoff post at corner (0.8, 0.8) cm, postH tall (welds into the plate).
	sPost := addSketchOn(t, cs)
	axisRect(sPost, [][]float64{{0.3, 0.3}, {1.3, 1.3}}, "postS", "postS")
	solved(sPost)
	postName := feat("3-post", "extrude", map[string]any{"sketchIndex": sPost, "profileIndex": 0, "distance": "postH", "operation": "join"})

	// 4. Replicate the post to all four corners (each join welds to the plate → still one body).
	// NOTE: patterning the post ALONE works; patterning a GROUP that includes the counterbore hole
	// scatters/disconnects the copies (Oblikovati/Oblikovati#128), so we pattern post and hole
	// SEPARATELY, the same order the box-bosses test uses.
	feat("4-post-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{postName}, "countX": 2, "countY": 2,
		"stepX": []float64{2.4, 0, 0}, "stepY": []float64{0, 2.4, 0},
	})

	// 5. Counterbore screw hole down one post top, then 6. pattern it onto the other three posts.
	holeName := feat("5-hole", "hole", map[string]any{
		"faceRef": topFaceKey(t, cs), "type": "counterbore",
		"diameter": "boreD", "depth": "10 mm", "counterDiameter": "cbD", "counterDepth": "3 mm",
	})
	feat("6-hole-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{holeName}, "countX": 2, "countY": 2,
		"stepX": []float64{2.4, 0, 0}, "stepY": []float64{0, 2.4, 0},
	})

	if got := partVolume(t, cs); got <= 0 || got > 4.0*4.0*1.3 {
		t.Errorf("standoff-mount volume = %.4f cm^3, out of range", got)
	}
	_ = math.Pi
}
