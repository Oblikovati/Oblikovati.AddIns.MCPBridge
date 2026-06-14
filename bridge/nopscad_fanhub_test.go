// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopFanWithHub re-models a 50 mm fan (NopSCADlib vitamins/fan.scad) as ONE feature-based part of
// EIGHT stacked features, to exercise how later features interact with the geometry earlier ones
// produced. Every sketch is fully constrained to 0 DOF and driven by named parameters (the Inventor
// way); the body is re-validated (one manifold/closed/oriented solid) after EVERY feature:
//
//	frame extrude(depth) → corner fillet(fr) → central bore(boreD, → ring) → H strut(strutW, join) →
//	V strut(join) → hub(hubD, join) → corner hole(holeD) → rectangular pattern of the hole (×4)
//
// The interaction that matters: the struts must weld to a filleted+bored ring, the hub must weld onto
// the strut cross, and the patterned holes must cut the merged result.
func TestNopFanWithHub(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	for _, p := range [][2]string{
		{"fw", "50 mm"}, {"depth", "10 mm"}, {"fr", "5 mm"}, {"boreD", "47 mm"},
		{"strutW", "3 mm"}, {"hubD", "10 mm"}, {"holeD", "3.4 mm"},
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
	// axisRect constrains a corner-rectangle as an axis-aligned box grounded at its first corner, sized
	// by the wExpr×hExpr parameters → 0 DOF.
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
	groundedCircle := func(si int, cx, cy float64, diamExpr string) {
		t.Helper()
		c := idsOf(t, cs, map[string]any{"sketchIndex": si, "kind": "circle", "points": [][]float64{{cx, cy}}, "radius": "0.2 cm"})
		con(si, "ground", c[1])
		dim(si, "diameter", diamExpr, c[0])
	}
	mustValid := func(step string) {
		t.Helper()
		part, err := modelaccess.ActivePart(s)
		if err != nil {
			t.Fatalf("%s: active part: %v", step, err)
		}
		bodies := part.SurfaceBodies().All()
		if len(bodies) != 1 {
			t.Fatalf("%s: want 1 body, got %d (a join left the part disconnected)", step, len(bodies))
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

	// 1. fw×fw frame, centred on the origin (so the bore/struts/hub sit at 0,0).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	rect := rectFull(t, cs, [][]float64{{-2.5, -2.5}, {2.5, 2.5}})
	bl, br, tr, tl := rect.points[0], rect.points[1], rect.points[2], rect.points[3]
	con(0, "ground", bl)
	con(0, "horizontal", bl, br)
	con(0, "horizontal", tl, tr)
	con(0, "vertical", bl, tl)
	con(0, "vertical", br, tr)
	dim(0, "distance", "fw", bl, br)
	dim(0, "distance", "fw", bl, tl)
	solved(0)
	feat("1-frame", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "depth", "operation": "new"})

	// 2. Round the four vertical corner edges.
	var corners []string
	for _, e := range bodyEdges(t, cs) {
		if math.Abs(e.point[0]) > 2.0 && math.Abs(e.point[1]) > 2.0 {
			corners = append(corners, e.key)
		}
	}
	if len(corners) < 4 {
		t.Fatalf("want 4 vertical corner edges, found %d", len(corners))
	}
	feat("2-fillet", "fillet", map[string]any{"edgeRefs": corners[:4], "radius": "fr"})

	// 3. Central bore → the frame becomes a ring.
	sBore := addSketchOn(t, cs)
	groundedCircle(sBore, 0, 0, "boreD")
	solved(sBore)
	feat("3-bore", "extrude", map[string]any{"sketchIndex": sBore, "profileIndex": 0, "extent": "through-all", "operation": "cut"})

	// 4 & 5. Two crossing struts spanning the full width (weld to the ring on both ends).
	sH := addSketchOn(t, cs)
	axisRect(sH, [][]float64{{-2.5, -0.15}, {2.5, 0.15}}, "fw", "strutW")
	solved(sH)
	feat("4-strutH", "extrude", map[string]any{"sketchIndex": sH, "profileIndex": 0, "distance": "depth", "operation": "join"})

	sV := addSketchOn(t, cs)
	axisRect(sV, [][]float64{{-0.15, -2.5}, {0.15, 2.5}}, "strutW", "fw")
	solved(sV)
	feat("5-strutV", "extrude", map[string]any{"sketchIndex": sV, "profileIndex": 0, "distance": "depth", "operation": "join"})

	// 6. Central hub joined onto the strut cross.
	sHub := addSketchOn(t, cs)
	groundedCircle(sHub, 0, 0, "hubD")
	solved(sHub)
	feat("6-hub", "extrude", map[string]any{"sketchIndex": sHub, "profileIndex": 0, "distance": "depth", "operation": "join"})

	// 7. One corner mounting hole, then 8. pattern it to all four corners.
	sHole := addSketchOn(t, cs)
	groundedCircle(sHole, 2, 2, "holeD")
	solved(sHole)
	holeName := feat("7-hole", "extrude", map[string]any{"sketchIndex": sHole, "profileIndex": 0, "extent": "through-all", "operation": "cut"})
	feat("8-pattern", "patternRectangular", map[string]any{
		"sourceFeatures": []string{holeName}, "countX": 2, "countY": 2,
		"stepX": []float64{-4, 0, 0}, "stepY": []float64{0, -4, 0},
	})

	if got := partVolume(t, cs); got <= 0 || got > 5.0*5.0*1.0 {
		t.Errorf("fan-with-hub volume = %.4f cm^3, out of range (0, 25)", got)
	}
}

// lastFeatureName returns the name of the most recently added feature (model.tree order) — used to feed
// a feature into a pattern by name without hardcoding the auto-generated index.
func lastFeatureName(t *testing.T, cs *mcp.ClientSession) string {
	t.Helper()
	var mt struct {
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
	}
	callJSON(t, cs, "get_model_tree", nil, &mt)
	if len(mt.Features) == 0 {
		t.Fatal("get_model_tree returned no features")
	}
	return mt.Features[len(mt.Features)-1].Name
}
