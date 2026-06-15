// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/addin/modelaccess"
	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
)

// partBuilder bundles the session, client and the small authoring helpers shared by the
// PT_camera vitamin parts (the geared stepper and the Pi camera), so each part is modelled
// once and re-used both by its own validation test and by the assembly that places it.
type partBuilder struct {
	t  *testing.T
	s  *app.Session
	cs *mcp.ClientSession
}

func (b *partBuilder) param(name, expr string) {
	callJSON(b.t, b.cs, "add_parameter", map[string]any{"name": name, "expression": expr}, nil)
}

func (b *partBuilder) con(si int, kind string, ents ...uint64) {
	callJSON(b.t, b.cs, "add_sketch_constraint", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents}, nil)
}

func (b *partBuilder) dim(si int, kind, expr string, ents ...uint64) {
	callJSON(b.t, b.cs, "add_sketch_dimension", map[string]any{"sketchIndex": si, "kind": kind, "entities": ents, "expression": expr}, nil)
}

func (b *partBuilder) solved(si int) {
	b.t.Helper()
	var sv struct {
		DOF int `json:"dof"`
	}
	callJSON(b.t, b.cs, "solve_sketch", map[string]any{"sketchIndex": si}, &sv)
	if sv.DOF != 0 {
		b.t.Fatalf("sketch %d not fully constrained: dof=%d, want 0", si, sv.DOF)
	}
}

// mustValid asserts the active part holds exactly one manifold/closed/oriented solid.
func (b *partBuilder) mustValid(step string) {
	b.t.Helper()
	part, err := modelaccess.ActivePart(b.s)
	if err != nil {
		b.t.Fatalf("%s: active part: %v", step, err)
	}
	bodies := part.SurfaceBodies().All()
	if len(bodies) != 1 {
		b.t.Fatalf("%s: want 1 body, got %d (a join/cut disconnected the part)", step, len(bodies))
	}
	if r := ops.Validate(bodies[0]); !r.Valid {
		b.t.Fatalf("%s: INVALID (manifold=%v closed=%v orient=%v): %v",
			step, r.Manifold, r.Closed, r.OrientationOK, capIssues(r.Issues))
	}
}

func (b *partBuilder) feat(step, kind string, args map[string]any) string {
	b.t.Helper()
	if h, reason := applyFeature(b.t, b.cs, kind, args); !h {
		b.t.Fatalf("%s: feature %q unhealthy: %s", step, kind, reason)
	}
	b.mustValid(step)
	return lastFeatureName(b.t, b.cs)
}

// circle draws a grounded circle in sketch si at (cx,cy) cm and returns its [circle, center] ids.
func (b *partBuilder) circle(si int, cx, cy float64, r string) []uint64 {
	c := idsOf(b.t, b.cs, map[string]any{"sketchIndex": si, "kind": "circle", "points": [][]float64{{cx, cy}}, "radius": r})
	b.con(si, "ground", c[1])
	return c
}

// rectFC draws a fully-constrained rectangle (ground one corner, the four edge constraints, and
// width/height dimensions) in sketch si and returns its corners.
func (b *partBuilder) rectFC(si int, pts [][]float64, wExpr, hExpr string) {
	r := rectOn(b.t, b.cs, si, pts)
	bl, br, tr, tl := r.points[0], r.points[1], r.points[2], r.points[3]
	b.con(si, "ground", bl)
	b.con(si, "horizontal", bl, br)
	b.con(si, "horizontal", tl, tr)
	b.con(si, "vertical", bl, tl)
	b.con(si, "vertical", br, tr)
	b.dim(si, "distance", wExpr, bl, br)
	b.dim(si, "distance", hExpr, bl, tl)
	b.solved(si)
}

// workPlane creates an offset work plane from an origin plane (e.g. "origin/plane/yz") and
// returns its index.
func (b *partBuilder) workPlane(ref, offset string) int {
	var wp struct {
		Index int `json:"index"`
	}
	callJSON(b.t, b.cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{ref}, "offset": offset}, &wp)
	return wp.Index
}

// sketchOn creates a sketch on the given work plane and returns its index.
func (b *partBuilder) sketchOn(wp int) int {
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(b.t, b.cs, "create_sketch", map[string]any{"workPlaneIndex": wp}, &sk)
	return sk.SketchIndex
}

// newPartDoc creates and activates a fresh part document and returns its id.
func newPartDoc(t *testing.T, cs *mcp.ClientSession, name string) uint64 {
	t.Helper()
	var doc struct {
		ID uint64 `json:"id"`
	}
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": name}, &doc)
	callJSON(t, cs, "activate_document", map[string]any{"id": doc.ID}, nil)
	return doc.ID
}

// buildStepperPart models the 28BYJ-48 geared stepper into the active part (its shaft axis is
// the Z axis at the origin — the rotation axis the assembly joints about). See
// TestNopGearedStepper28BYJ48 for the dimensional/validity assertions.
func buildStepperPart(b *partBuilder) {
	for _, p := range [][2]string{
		{"can_dia", "28 mm"}, {"can_h", "19 mm"}, {"rim_r", "1 mm"}, {"shaft_off", "8 mm"},
		{"boss_dia", "9 mm"}, {"boss_proj", "1.5 mm"}, {"shaft_dia", "5 mm"}, {"shaft_len", "10 mm"},
		{"flat_len", "6 mm"}, {"shaft_flat", "3 mm"}, {"screw_pitch", "35 mm"}, {"lug_w", "7 mm"},
		{"lug_t", "0.85 mm"}, {"hole_dia", "4.2 mm"}, {"wire_w", "14.7 mm"}, {"wire_dep", "6 mm"},
		{"wire_h", "16.5 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t

	// 1. Tin can Ø28×19, centre offset 8 mm from the shaft axis (the shaft is at the origin).
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]
	can := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, -0.8}}, "radius": "1.4 cm"})
	b.con(0, "ground", o)
	b.con(0, "vertical", o, can[1])
	b.dim(0, "distance", "shaft_off", o, can[1])
	b.dim(0, "diameter", "can_dia", can[0])
	b.solved(0)
	b.feat("1-can", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "can_h", "operation": "new"})

	// 2. Boss Ø9 stub projecting 1.5 mm below the base (top cap coplanar with the can bottom).
	sBoss := addSketchOn(t, cs)
	b.dim(sBoss, "diameter", "boss_dia", b.circle(sBoss, 0, 0, "0.45 cm")[0])
	b.solved(sBoss)
	b.feat("2-boss", "extrude", map[string]any{"sketchIndex": sBoss, "profileIndex": 0, "distance": "boss_proj", "direction": "negative", "operation": "join"})

	// 3. Output shaft Ø5 round, 10 mm down the −Z axis, join onto the boss.
	sShaft := addSketchOn(t, cs)
	b.dim(sShaft, "diameter", "shaft_dia", b.circle(sShaft, 0, 0, "0.25 cm")[0])
	b.solved(sShaft)
	b.feat("3-shaft", "extrude", map[string]any{"sketchIndex": sShaft, "profileIndex": 0, "distance": "shaft_len", "direction": "negative", "operation": "join"})

	// 4. The two tip flats (the D-section) on a work plane 4 mm below the base.
	var fwp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "-4 mm"}, &fwp)
	for i, ypts := range [][][]float64{{{-0.5, 0.15}, {0.5, 0.5}}, {{-0.5, -0.5}, {0.5, -0.15}}} {
		var sk struct {
			SketchIndex int `json:"sketchIndex"`
		}
		callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": fwp.Index}, &sk)
		b.rectFC(sk.SketchIndex, ypts, "2 * shaft_dia", "shaft_dia - shaft_flat / 2")
		b.feat(stepName("4-flat", i), "extrude", map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": "flat_len", "direction": "negative", "operation": "cut"})
	}

	// 5. Mounting-lug plate 42×7×0.85 on the base plane, join; 6. two Ø4.2 screw holes, cut.
	sLug := addSketchOn(t, cs)
	b.rectFC(sLug, [][]float64{{-2.1, -1.15}, {2.1, -0.45}}, "screw_pitch + lug_w", "lug_w")
	b.feat("5-lug", "extrude", map[string]any{"sketchIndex": sLug, "profileIndex": 0, "distance": "lug_t", "direction": "negative", "operation": "join"})

	sHoles := addSketchOn(t, cs)
	b.dim(sHoles, "diameter", "hole_dia", b.circle(sHoles, 1.75, -0.8, "0.21 cm")[0])
	b.dim(sHoles, "diameter", "hole_dia", b.circle(sHoles, -1.75, -0.8, "0.21 cm")[0])
	b.solved(sHoles)
	b.feat("6-holeR", "extrude", map[string]any{"sketchIndex": sHoles, "profileIndex": 0, "distance": "lug_t", "direction": "negative", "operation": "cut"})
	b.feat("6-holeL", "extrude", map[string]any{"sketchIndex": sHoles, "profileIndex": 1, "distance": "lug_t", "direction": "negative", "operation": "cut"})

	// 7. Wire connector block 14.7×6×16.5 behind the can, join.
	sWire := addSketchOn(t, cs)
	b.rectFC(sWire, [][]float64{{-0.735, -2.5}, {0.735, -1.9}}, "wire_w", "wire_dep")
	b.feat("7-wire", "extrude", map[string]any{"sketchIndex": sWire, "profileIndex": 0, "distance": "wire_h", "operation": "join"})

	// 8. Round the can's top rim r1 (last, cosmetic — union onto a filleted body is the gap).
	b.feat("8-rim", "fillet", map[string]any{"edgeRefs": []string{topRimEdgeKey(t, cs)}, "radius": "rim_r"})
}

// buildCameraPart models the Raspberry Pi camera V1 into the active part (the lens points up
// +Z; the part centre is the PCB centre). See TestNopRpiCameraV1 for the assertions.
func buildCameraPart(b *partBuilder) {
	for _, p := range [][2]string{
		{"pcb_l", "25 mm"}, {"pcb_w", "24 mm"}, {"pcb_t", "1 mm"}, {"hole_d", "2.1 mm"},
		{"lens_base", "8 mm"}, {"lens_h", "3 mm"}, {"barrel_d", "7.5 mm"}, {"barrel_h", "4 mm"},
		{"top_d", "5.5 mm"}, {"top_h", "5 mm"}, {"ap_d", "2 mm"}, {"ap_h", "1 mm"},
		{"conn_w", "8 mm"}, {"conn_d", "5 mm"}, {"conn_h", "1 mm"},
	} {
		b.param(p[0], p[1])
	}
	cs, t := b.cs, b.t

	// 1. PCB 25×24×1, top face at z=0 (grows down); 2. four Ø2.1 mounting holes, cut through.
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	b.rectFC(0, [][]float64{{-1.25, -1.2}, {1.25, 1.2}}, "pcb_l", "pcb_w")
	b.feat("1-pcb", "extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "pcb_t", "direction": "negative", "operation": "new"})

	sHoles := addSketchOn(t, cs)
	for _, xy := range [][2]float64{{0.2, -0.2}, {-0.2, -0.2}, {0.2, 0.96}, {-0.2, 0.96}} {
		b.dim(sHoles, "diameter", "hole_d", b.circle(sHoles, xy[0], xy[1], "0.105 cm")[0])
	}
	b.solved(sHoles)
	for i := 0; i < 4; i++ {
		b.feat(stepName("2-hole", i), "extrude", map[string]any{"sketchIndex": sHoles, "profileIndex": i, "distance": "pcb_t", "direction": "negative", "operation": "cut"})
	}

	// 3. Lens base block 8×8×3 at the lens offset (0,−2.4), join.
	sBase := addSketchOn(t, cs)
	b.rectFC(sBase, [][]float64{{-0.4, -0.64}, {0.4, 0.16}}, "lens_base", "lens_base")
	b.feat("3-lensbase", "extrude", map[string]any{"sketchIndex": sBase, "profileIndex": 0, "distance": "lens_h", "operation": "join"})

	// 4. Lens barrel Ø7.5×4 and 5. lens top Ø5.5×5, concentric on the block, join.
	sBarrel := addSketchOn(t, cs)
	b.dim(sBarrel, "diameter", "barrel_d", b.circle(sBarrel, 0, -0.24, "0.375 cm")[0])
	b.solved(sBarrel)
	b.feat("4-barrel", "extrude", map[string]any{"sketchIndex": sBarrel, "profileIndex": 0, "distance": "barrel_h", "operation": "join"})

	sTop := addSketchOn(t, cs)
	b.dim(sTop, "diameter", "top_d", b.circle(sTop, 0, -0.24, "0.275 cm")[0])
	b.solved(sTop)
	b.feat("5-lenstop", "extrude", map[string]any{"sketchIndex": sTop, "profileIndex": 0, "distance": "top_h", "operation": "join"})

	// 6. Aperture: a Ø2 bore 1 mm into the top of the lens (work plane at its top, z=5 mm).
	var awp struct {
		Index int `json:"index"`
	}
	callJSON(t, cs, "create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "5 mm"}, &awp)
	var ask struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"workPlaneIndex": awp.Index}, &ask)
	b.dim(ask.SketchIndex, "diameter", "ap_d", b.circle(ask.SketchIndex, 0, -0.24, "0.1 cm")[0])
	b.solved(ask.SketchIndex)
	b.feat("6-aperture", "extrude", map[string]any{"sketchIndex": ask.SketchIndex, "profileIndex": 0, "distance": "ap_h", "direction": "negative", "operation": "cut"})

	// 7. Flex connector block 8×5×1 at (0,8) on the PCB top, join.
	sConn := addSketchOn(t, cs)
	b.rectFC(sConn, [][]float64{{-0.4, 0.55}, {0.4, 1.05}}, "conn_w", "conn_d")
	b.feat("7-connector", "extrude", map[string]any{"sketchIndex": sConn, "profileIndex": 0, "distance": "conn_h", "operation": "join"})
}
