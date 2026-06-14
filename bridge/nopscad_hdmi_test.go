// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// keystoneD is the HDMI D() cross-section (hull of the two stacked rectangles) in cm, CCW.
var keystoneD = [][]float64{
	{-0.7, 0.6}, {-0.7, 0.3}, {-0.5, 0.15}, {0.5, 0.15}, {0.7, 0.3}, {0.7, 0.6},
}

// TestNopHdmi models the NopSCADlib HDMI socket metal shell (vitamins/pcb.scad hdmi(),
// hdmi_full) the Inventor way. The D() cross-section is the convex hull of two stacked
// rectangles ⇒ a keystone hexagon, authored as a closed polyline. offset_sketch grows it by the
// wall thickness t (rounded, as OpenSCAD offset() does); the wall band (annulus) is extruded the
// full depth into a hollow tube, then the inner-disk profile is JOIN-extruded 1 mm to plug one
// end — the solid end flange. This stacks two extrudes whose geometry collides at the shared
// inner walls (coplanar union) — exactly the kind of feature interaction that simpler parts
// don't exercise.
//
// hdmi_full = [l=12, iw1=14, iw2=10, ih1=3, ih2=4.5, h=6.5, t=0.5] (mm). Volume target:
// band_area·l + inner_area·cap, band_area = perim·t + π·t² (Minkowski ring of the convex
// hexagon, exact for the rounded offset). inner_area = 60 mm², perim = 35 mm.
func TestNopHdmi(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	// Keystone hexagon D() in cm (mm/10), CCW.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": 0, "kind": "polyline", "closed": true, "points": keystoneD,
	}, nil)
	if closedProfileIndex(t, cs) < 0 {
		t.Fatal("keystone polyline did not form a closed profile")
	}

	// Grow by the wall thickness t = 0.5 mm (rounded corners) ⇒ outer keystone loop.
	var off struct {
		Created []uint64 `json:"created"`
	}
	callJSON(t, cs, "offset_sketch", map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "0.5 mm", "arcSegments": 16,
	}, &off)
	if len(off.Created) < 6 {
		t.Fatalf("region offset created %d entities, want a closed outer loop", len(off.Created))
	}

	// The wall band (annulus between the two keystones) → hollow tube, full depth 12 mm.
	band := profileWithHole(t, cs)
	if band < 0 {
		t.Fatal("no annular (wall band) profile after offset")
	}
	if h, r := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 0, "profileIndex": band, "distance": "12 mm", "operation": "new"}); !h {
		t.Fatalf("wall extrude unhealthy: %s", r)
	}

	// A second sketch of the bare keystone → 1 mm JOIN cap plugging one end (the metal flange).
	// Its solid disk unions with the hollow tube along the shared inner walls (coplanar union).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{
		"sketchIndex": 1, "kind": "polyline", "closed": true, "points": keystoneD,
	}, nil)
	if h, r := applyFeature(t, cs, "extrude",
		map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "1 mm", "operation": "join"}); !h {
		t.Fatalf("cap extrude unhealthy: %s", r)
	}

	// One valid solid: the union of the hollow tube and the end cap.
	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 || !ops.Validate(bodies[0]).Valid {
		t.Fatalf("hdmi shell is not a single valid solid (bodies=%d)", len(bodies))
	}

	// Volume in cm³: band·depth + inner·cap.
	perim, tWall := 3.5, 0.05 // cm
	innerArea := 0.6          // cm²
	band2 := perim*tWall + math.Pi*tWall*tWall
	want := band2*1.2 + innerArea*0.1
	if got := partVolume(t, cs); math.Abs(got-want)/want > 0.03 {
		t.Errorf("hdmi shell volume = %.6f cm^3, want ~%.6f", got, want)
	}
}
