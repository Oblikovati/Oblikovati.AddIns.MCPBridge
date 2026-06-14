// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/geom"
	"oblikovati.org/kernel/ops"
	"oblikovati.org/kernel/topo"
)

// TestNopThreadedSpacerThreaded models a NopSCADlib-style threaded spacer and is the #129 step-2
// proof: a fully constrained, parameter-driven annular profile REVOLVED 360° into a tube, then the
// bore THREADED. The revolve emits a true analytic cylinder for the bore (brep.SolidOfRevolution),
// so `thread` attaches and the cut succeeds — where before it failed with `face is not cylindrical`
// (the revolve was a faceted prism). This was previously a red SPEC pinning that gap; it now asserts
// the gap is CLOSED.
func TestNopThreadedSpacerThreaded(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	for _, p := range [][2]string{{"od", "12 mm"}, {"id", "5 mm"}, {"h", "20 mm"}} {
		callJSON(t, cs, "add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}

	// Annular profile on XZ: inner edge grounded at radius id/2, wall (od−id)/2 wide, h tall.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XZ"}, nil)
	var r struct {
		PointIDs []uint64 `json:"pointIds"`
	}
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": [][]float64{{0.25, 0}, {0.6, 2.0}}}, &r)
	bl, br, tr, tl := r.PointIDs[0], r.PointIDs[1], r.PointIDs[2], r.PointIDs[3]
	con("ground", bl)
	con("horizontal", bl, br)
	con("horizontal", tl, tr)
	con("vertical", bl, tl)
	con("vertical", br, tr)
	dim("distance", "(od - id) / 2", bl, br)
	dim("distance", "h", bl, tl)
	var sv struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &sv)
	if sv.DOF != 0 {
		t.Fatalf("spacer profile not fully constrained: dof=%d, want 0", sv.DOF)
	}

	// Revolve → tube. This WORKS: a valid one-body solid.
	if h, reason := applyFeature(t, cs, "revolve", map[string]any{"sketchIndex": 0, "profileIndex": 0, "axisRef": "origin/axis/z", "angle": "360 deg", "operation": "new"}); !h {
		t.Fatalf("revolve unhealthy: %s", reason)
	}
	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	if r := ops.Validate(part.SurfaceBodies().All()[0]); !r.Valid {
		t.Fatalf("revolved tube is INVALID: %v", capIssues(r.Issues))
	}

	// #129 step 2: thread the bore. The analytic revolve gives the bore a true cylindrical face (the
	// inner cylinder, radius id/2 = 0.25 cm), so the thread now attaches and cuts successfully. A full
	// periodic cylinder's range-box centre sits ON the axis, so select it by its analytic radius
	// rather than a representative point.
	bore := cylinderFaceKeyByRadius(t, part.SurfaceBodies().All()[0], 0.25)
	if h, reason := applyFeature(t, cs, "thread", map[string]any{"faceRef": bore, "designation": "M5x0.8", "cut": true}); !h {
		t.Fatalf("thread on the revolved bore is unhealthy (#129 step 2 regressed?): %s", reason)
	}
	if r := ops.Validate(part.SurfaceBodies().All()[0]); !r.Valid {
		t.Fatalf("threaded spacer is INVALID: %v", capIssues(r.Issues))
	}
}

// cylinderFaceKeyByRadius returns the reference key of the body's analytic cylinder face whose
// radius is ~r (cm) — the way to pick a specific cylindrical wall (e.g. a bore) of a solid of
// revolution, whose periodic faces have an on-axis range-box centre no representative point can
// distinguish.
func cylinderFaceKeyByRadius(t *testing.T, b *topo.Body, r float64) string {
	t.Helper()
	for _, f := range b.Faces() {
		if cyl, ok := f.Geometry().(geom.Cylinder); ok && math.Abs(cyl.Radius-r) < 0.03 {
			return string(f.ReferenceKey())
		}
	}
	t.Fatalf("no analytic cylinder face of radius %g", r)
	return ""
}
