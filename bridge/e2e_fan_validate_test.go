// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
	"oblikovati.org/kernel/topo"
)

// TestFanBodyStaysManifold reproduces the parametric fan and runs the kernel validator
// (ops.Validate: manifold, closed, consistent orientation) on every body after every feature —
// to localise the reported viewport deformity (deformed mesh / wrong normals / non-manifold) to
// the exact step that first produces an invalid B-rep. A deformed tessellation comes from a body
// that is non-manifold (an edge shared by >2 faces, e.g. blades meeting at the hub), open (a
// hole the boolean left), or inconsistently oriented (flipped normals) — all of which this
// asserts step by step, with face/edge/vertex counts logged for the failing one.
func TestFanBodyStaysManifold(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	check := func(step string) {
		t.Helper()
		part, err := modelaccess.ActivePart(s)
		if err != nil {
			t.Fatalf("%s: active part: %v", step, err)
		}
		bodies := part.SurfaceBodies().All()
		for i, b := range bodies {
			r := ops.Validate(b)
			v, e, f := len(b.Vertices()), len(b.Edges()), len(b.Faces())
			euler := v - e + f
			if !r.Valid {
				t.Errorf("%s: body[%d] INVALID manifold=%v closed=%v orient=%v V%d-E%d+F%d=%d issues=%v",
					step, i, r.Manifold, r.Closed, r.OrientationOK, v, e, f, euler, capIssues(r.Issues))
			} else {
				t.Logf("%s: body[%d] ok  V%d-E%d+F%d=%d (euler 2 ⇒ sphere-topology solid)", step, i, v, e, f, euler)
			}
		}
		if len(bodies) != 1 {
			t.Logf("%s: %d bodies", step, len(bodies))
		}
	}

	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2.5, 2.5}}}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "15 mm", "operation": "new"})
	check("1-frame")

	var corners []string
	for _, e := range bodyEdges(t, cs) {
		if abs(e.point[0]) > 2.0 && abs(e.point[1]) > 2.0 {
			corners = append(corners, e.key)
		}
	}
	applyFeature(t, cs, "fillet", map[string]any{"edgeRefs": corners[:4], "radius": "5 mm"})
	check("2-fillet")

	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "23.5 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s1, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
	check("3-bore")

	s2 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s2, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "6.75 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s2, "profileIndex": 0, "distance": "15 mm", "operation": "join"})
	check("4-hub")

	root := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": root, "kind": "rectangle", "points": [][]float64{{0.6, -0.08}, {2.35, 0.08}}}, nil)
	var tipWP struct {
		Index   int  `json:"index"`
		Healthy bool `json:"healthy"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "14 mm"}, &tipWP)
	var tipSk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": tipWP.Index}, &tipSk)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": tipSk.SketchIndex, "kind": "rectangle", "variant": "threePoint",
		"points": [][]float64{{0.591, 0.130}, {2.236, 0.729}, {2.182, 0.879}}}, nil)
	blade, _, _ := addNamedFeature(t, cs, "loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": root, "profileIndex": 0}, {"sketchIndex": tipSk.SketchIndex, "profileIndex": 0}},
		"operation": "join",
	})
	check("5-blade-loft")

	applyFeature(t, cs, "patternCircular", map[string]any{
		"sourceFeatures": []string{blade}, "count": 7, "angle": "360 deg", "axisDir": []float64{0, 0, 1},
	})
	check("6-blade-pattern")

	s3 := addSketchOn(t, cs)
	for _, c := range [][2]float64{{2, 2}, {-2, 2}, {-2, -2}, {2, -2}} {
		callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s3, "kind": "circle", "points": [][]float64{{c[0], c[1]}}, "radius": "1.7 mm"}, nil)
	}
	for pi := 0; pi < 4; pi++ {
		applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s3, "profileIndex": pi, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
	}
	check("7-mounts")
}

// capIssues trims the validator's issue list so a flood of non-manifold edges does not bury the
// log; the count tells the scale, the first few tell the shape.
func capIssues(issues []string) []string {
	const max = 6
	if len(issues) <= max {
		return issues
	}
	return append(issues[:max:max], "…")
}

var _ = topo.NewSurfaceBodies // keep the topo import even if the helper set narrows

// TestLoftBladeAloneIsManifold isolates the fan defect to sweptSolid vs the join boolean: it
// builds the SAME blade loft as a standalone NEW body (no boolean) and validates it. If this is
// already non-manifold, the loft solid construction (sweptSolid) is the bug; if it is a valid
// solid here but the in-fan join is not, the loft→join boolean is the bug.
func TestLoftBladeAloneIsManifold(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	root := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": root, "kind": "rectangle", "points": [][]float64{{0.6, -0.08}, {2.35, 0.08}}}, nil)
	var tipWP struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "14 mm"}, &tipWP)
	var tipSk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": tipWP.Index}, &tipSk)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": tipSk.SketchIndex, "kind": "rectangle", "variant": "threePoint",
		"points": [][]float64{{0.591, 0.130}, {2.236, 0.729}, {2.182, 0.879}}}, nil)
	if _, h, r := addNamedFeature(t, cs, "loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": root, "profileIndex": 0}, {"sketchIndex": tipSk.SketchIndex, "profileIndex": 0}},
		"operation": "new",
	}); !h {
		t.Fatalf("standalone loft unhealthy: %s", r)
	}

	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	for i, b := range part.SurfaceBodies().All() {
		rep := ops.Validate(b)
		v, e, f := len(b.Vertices()), len(b.Edges()), len(b.Faces())
		if !rep.Valid {
			t.Errorf("loft body[%d] INVALID manifold=%v closed=%v orient=%v V%d-E%d+F%d=%d issues=%v",
				i, rep.Manifold, rep.Closed, rep.OrientationOK, v, e, f, v-e+f, capIssues(rep.Issues))
		} else {
			t.Logf("loft body[%d] ok  V%d-E%d+F%d=%d", i, v, e, f, v-e+f)
		}
	}
}

// TestBladeJoinBooleanIsTheDefect is the #860 regression: it builds the body through the hub,
// builds the blade as a SEPARATE solid, then joins the two with ops.Boolean directly and
// validates. The blade bottom is coplanar with the body bottom and the blade's outer end crosses
// the CONCAVE faceted bore wall (a partial penetration of a re-entrant faceted surface). Until
// the arrangement-robustness fixes (collinear-edge imprint capture, coplanar imprint
// material-clip, T-junction welding, filled-hole merge) this welded the blade as a coincident
// non-manifold shell; it must now come back a clean manifold solid.
func TestBladeJoinBooleanIsTheDefect(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	if _, err := s.NewPart(); err != nil {
		t.Fatalf("NewPart: %v", err)
	}
	cs := e2eClient(t, s)

	// Body through the hub (the valid step-4 solid).
	s0 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s0, "kind": "rectangle", "variant": "center", "points": [][]float64{{0, 0}, {2.5, 2.5}}}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s0, "profileIndex": 0, "distance": "15 mm", "operation": "new"})
	s1 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s1, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "23.5 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s1, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
	s2 := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s2, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "6.75 mm"}, nil)
	applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": s2, "profileIndex": 0, "distance": "15 mm", "operation": "join"})

	// The blade as a SEPARATE solid (operation new ⇒ a second body).
	root := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": root, "kind": "rectangle", "points": [][]float64{{0.6, -0.08}, {2.35, 0.08}}}, nil)
	var tipWP struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "14 mm"}, &tipWP)
	var tipSk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": tipWP.Index}, &tipSk)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": tipSk.SketchIndex, "kind": "rectangle", "variant": "threePoint",
		"points": [][]float64{{0.591, 0.130}, {2.236, 0.729}, {2.182, 0.879}}}, nil)
	addNamedFeature(t, cs, "loft", map[string]any{
		"sections":  []map[string]any{{"sketchIndex": root, "profileIndex": 0}, {"sketchIndex": tipSk.SketchIndex, "profileIndex": 0}},
		"operation": "new",
	})

	part, err := modelaccess.ActivePart(s)
	if err != nil {
		t.Fatalf("active part: %v", err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 2 {
		t.Fatalf("want 2 bodies (hub-solid + blade), got %d", len(bodies))
	}
	for i, b := range bodies {
		if r := ops.Validate(b); !r.Valid {
			t.Fatalf("input body[%d] already invalid: %v", i, capIssues(r.Issues))
		}
	}
	joined, err := ops.Boolean(ops.Join, bodies[0], bodies[1])
	if err != nil {
		t.Fatalf("join err: %v", err)
	}
	r := ops.Validate(joined)
	v, e, f := len(joined.Vertices()), len(joined.Edges()), len(joined.Faces())
	t.Logf("JOIN result: valid=%v manifold=%v closed=%v V%d-E%d+F%d=%d (inputs: body0 F%d + blade F%d = %d)",
		r.Valid, r.Manifold, r.Closed, v, e, f, v-e+f, len(bodies[0].Faces()), len(bodies[1].Faces()), len(bodies[0].Faces())+len(bodies[1].Faces()))
	if !r.Valid {
		t.Errorf("blade JOIN is non-manifold/open (the deformed mesh): %v", capIssues(r.Issues))
	}
}
