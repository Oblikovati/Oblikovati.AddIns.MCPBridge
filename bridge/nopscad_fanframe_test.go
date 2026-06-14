// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopFanFrameFillet models the FRAME of a 50 mm fan (NopSCADlib vitamins/fan.scad) as a
// standalone solid the Inventor way: a square plate, its four vertical corners rounded with a
// 3D fillet, a large central bore, and four corner mounting holes. It is a real constituent of
// the fan (the part minus the hub+blades, which need the loft the boolean can't yet take at the
// re-entrant bore corner — the V2 defect). This rung climbs toward the fan by exercising the
// FILLET feature on real geometry (previously only inside the skipped fan spec) plus a big bore
// and a hole pattern — all planar/clean, so it stays a valid manifold solid throughout.
//
// In-proc so it can run ops.Validate after the (fragile) fillet and each boolean.
func TestNopFanFrameFillet(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

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

	// 50×50×15 mm square plate (centered on the origin).
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2.5, 2.5}}}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "15 mm", "operation": "new"})
	mustValid("1-plate")

	// Round the four vertical corner edges (r=5 mm) — the fan's rounded frame.
	var corners []string
	for _, e := range bodyEdges(t, cs) {
		if math.Abs(e.point[0]) > 2.0 && math.Abs(e.point[1]) > 2.0 {
			corners = append(corners, e.key)
		}
	}
	if len(corners) < 4 {
		t.Fatalf("want 4 vertical corner edges, found %d", len(corners))
	}
	if h, reason := applyFeature(t, cs, "fillet", map[string]any{"edgeRefs": corners[:4], "radius": "5 mm"}); !h {
		t.Fatalf("corner fillet unhealthy: %s", reason)
	}
	mustValid("2-fillet")

	// Central bore (Ø47 mm) through the frame.
	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "23.5 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s1, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
	mustValid("3-bore")

	// Four Ø3.4 mm corner mounting holes (one sketch, four profiles).
	s2 := addSketchOn(t, cs)
	for _, c := range [][2]float64{{2, 2}, {-2, 2}, {-2, -2}, {2, -2}} {
		callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s2, "kind": "circle", "points": [][]float64{{c[0], c[1]}}, "radius": "1.7 mm"}, nil)
	}
	for pi := 0; pi < 4; pi++ {
		applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s2, "profileIndex": pi, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
	}
	mustValid("4-mounts")

	// Volume: plate − 4 fillet corners − bore − 4 mounting holes (cm^3; faceted bore ⇒ band).
	R, h := 0.5, 1.5 // fillet radius, plate thickness (cm)
	plate := 5.0*5.0*h - 4*(R*R-math.Pi*R*R/4)*h
	bore := math.Pi * 2.35 * 2.35 * h
	mounts := 4 * math.Pi * 0.17 * 0.17 * h
	want := plate - bore - mounts
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.03 {
		t.Errorf("fan-frame volume = %.5f cm^3, want ~%.5f (3%% faceting band)", got, want)
	}
}
