// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// TestNopThreadedBushExtrude proves the analytic-cylinder fix (Oblikovati/Oblikovati#129): a
// full-circle profile EXTRUDED yields a TRUE cylinder (an analytic cylindrical side face), so the
// THREAD feature attaches — which fails on the old faceted prism ("face is not cylindrical").
//
//	circle extrude(h) → assert exactly 1 cylinder face + 2 planar caps → thread the cylinder (cut)
func TestNopThreadedBushExtrude(t *testing.T) {

	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "od", "expression": "12 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "h", "expression": "15 mm"}, nil)

	// Fully-constrained circle: center grounded, diameter = od.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	circ := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "0.6 cm"})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{circ[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "diameter", "entities": []uint64{circ[0]}, "expression": "od"}, nil)
	var sv struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &sv)
	if sv.DOF != 0 {
		t.Fatalf("circle sketch not fully constrained: dof=%d", sv.DOF)
	}

	if h, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "h", "operation": "new"}); !h {
		t.Fatalf("extrude unhealthy: %s", reason)
	}
	if part, err := modelaccess.ActivePart(s); err != nil {
		t.Fatalf("active part: %v", err)
	} else if r := ops.Validate(part.SurfaceBodies().All()[0]); !r.Valid {
		t.Fatalf("extruded cylinder INVALID: %v", capIssues(r.Issues))
	}

	// The fix: the side wall is a single analytic cylinder face, not 64 planar facets.
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key  string `json:"key"`
				Kind string `json:"kind"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	callJSON(t, cs, "get_reference_keys", nil, &rk)
	planes, cyls, cylKey := 0, 0, ""
	for _, f := range rk.Bodies[0].Faces {
		switch f.Kind {
		case "cylinder":
			cyls++
			cylKey = f.Key
		case "plane":
			planes++
		}
	}
	if cyls != 1 || planes != 2 {
		t.Fatalf("extruded circle faces: %d cylinder + %d plane, want 1 cylinder (side) + 2 planes (caps) — #129", cyls, planes)
	}

	// THREAD the cylindrical wall — used to fail "face is not cylindrical".
	if h, reason := applyFeature(t, cs, "thread", map[string]any{"faceRef": cylKey, "designation": "M12x1.75", "cut": true}); !h {
		t.Fatalf("thread on the analytic cylinder unhealthy: %s (#129)", reason)
	}
	if part, err := modelaccess.ActivePart(s); err == nil {
		if r := ops.Validate(part.SurfaceBodies().All()[0]); !r.Valid {
			t.Fatalf("threaded cylinder INVALID: %v", capIssues(r.Issues))
		}
	}
}

// TestAnalyticCylinderCutPlanarizes covers the planarize-on-demand path (#129): an analytic cylinder
// cut by a slot must re-facet into a planar B-rep for the boolean (which cannot consume a full
// periodic cylinder face) and stay a valid solid.
func TestAnalyticCylinderCutPlanarizes(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	// Analytic cylinder (Ø20 × 20 mm).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "1 cm"})
	if h, r := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}); !h {
		t.Fatalf("extrude unhealthy: %s", r)
	}
	// Cut a rectangular slot through it — the boolean re-facets the cylinder (planarize) and survives.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {0.3, 1.2}}}, nil)
	if h, r := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "extent": "through-all", "operation": "cut"}); !h {
		t.Fatalf("slot cut unhealthy: %s (planarize-on-demand, #129)", r)
	}
	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 {
		t.Fatalf("want 1 body after cut, got %d", len(bodies))
	}
	if r := ops.Validate(bodies[0]); !r.Valid {
		t.Fatalf("cut cylinder INVALID: %v", capIssues(r.Issues))
	}
}
