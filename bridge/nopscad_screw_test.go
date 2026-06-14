// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNopCapScrewParametric models a socket-cap screw (NopSCADlib hs_cap) the Inventor way:
// a stepped half-section — head radius over shaft radius — drawn as a closed six-line profile,
// fully constrained (0 DOF) with the step width left to fall out of the head/shaft radii, then
// revolved 360° about the Y axis into the head+shaft solid. A parameter edit (lengthen the
// shaft) must flow through the DAG → solver → revolve and grow the volume.
//
// The hex drive socket is exercised by the CSG unit test (kernel/brep/nopscad_screw_test.go);
// here the focus is the stepped revolve and its parametric rebuild.
//
// Reference: NopSCADlib/vitamins/screw.scad (hs_cap) + screws.scad (M3_cap_screw: head Ø5.5,
// head h 3, shaft Ø3) at length 10 mm.
func TestNopCapScrewParametric(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "headD", "expression": "5.5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "headH", "expression": "3 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "shaftD", "expression": "3 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "len", "expression": "10 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Six lines forming the closed half-section (cm seeds; the dimensions below override them).
	// P0(0,.3)→P1(.275,.3)→P2(.275,0)→P3(.15,0)→P4(.15,-1)→P5(0,-1)→P0.
	mkLine := func(x0, y0, x1, y1 float64) []uint64 {
		return idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line",
			"points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l0 := mkLine(0, 0.3, 0.275, 0.3)   // head top
	l1 := mkLine(0.275, 0.3, 0.275, 0) // head side
	l2 := mkLine(0.275, 0, 0.15, 0)    // shoulder step
	l3 := mkLine(0.15, 0, 0.15, -1)    // shaft side
	l4 := mkLine(0.15, -1, 0, -1)      // bottom
	l5 := mkLine(0, -1, 0, 0.3)        // axis edge
	p := func(l []uint64) (uint64, uint64) { return l[1], l[2] }
	p0a, p0b := p(l0)
	p1a, p1b := p(l1)
	p2a, p2b := p(l2)
	p3a, p3b := p(l3)
	p4a, p4b := p(l4)
	p5a, p5b := p(l5)
	o := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0}}})[0]

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	// Chain the line endpoints into one closed loop.
	con("coincident", p0b, p1a)
	con("coincident", p1b, p2a)
	con("coincident", p2b, p3a)
	con("coincident", p3b, p4a)
	con("coincident", p4b, p5a)
	con("coincident", p5b, p0a)
	// Orientations: head top/shoulder/bottom horizontal; head side/shaft side/axis vertical.
	con("horizontal", p0a, p0b)
	con("vertical", p1a, p1b)
	con("horizontal", p2a, p2b)
	con("vertical", p3a, p3b)
	con("horizontal", p4a, p4b)
	con("vertical", p5a, p5b)
	// Anchor: world origin grounded; axis edge on the Y axis (P0.x = 0); shoulder on the X axis.
	con("ground", o)
	con("vertical", o, p0a)   // P0.x = 0 → with the axis edge vertical, P5.x = 0 too
	con("horizontal", o, p3a) // shoulder at y = 0

	dim := func(kind, expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents, "expression": expr}, nil)
	}
	dim("distance", "headD/2", p0a, p0b)  // head radius
	dim("distance", "headH", p1a, p1b)    // head height
	dim("distance", "len", p3a, p3b)      // shaft length
	dim("distance", "shaftD/2", p4a, p4b) // shaft radius (step width = headD/2 − shaftD/2 falls out)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("cap-screw section is not fully constrained: dof=%d, want 0", solve.DOF)
	}

	prof := closedProfileIndex(t, cs)
	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": prof, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}

	// Volume = head cylinder + shaft cylinder (cm^3; mm → cm).
	wantVol := func(headDmm, headHmm, shaftDmm, lenMM float64) float64 {
		hr, hh, sr, l := headDmm/20, headHmm/10, shaftDmm/20, lenMM/10
		return math.Pi*hr*hr*hh + math.Pi*sr*sr*l
	}
	if got, w := partVolume(t, cs), wantVol(5.5, 3, 3, 10); math.Abs(got-w)/w > 0.02 {
		t.Errorf("cap-screw volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Parametric: a longer shaft must add exactly its cylinder volume.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "len", "expression": "16 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(5.5, 3, 3, 16); math.Abs(got-w)/w > 0.02 {
		t.Errorf("lengthened cap-screw volume = %.6f cm^3, want ~%.6f", got, w)
	}
}

// closedProfileIndex returns the index of the first closed sketch profile (the revolve target).
func closedProfileIndex(t *testing.T, cs *mcp.ClientSession) int {
	t.Helper()
	var profs struct {
		Profiles []struct {
			Index  int  `json:"index"`
			Closed bool `json:"closed"`
		} `json:"profiles"`
	}
	callStructured(t, cs, "list_sketch_profiles", map[string]any{"sketchIndex": 0}, &profs)
	for _, p := range profs.Profiles {
		if p.Closed {
			return p.Index
		}
	}
	return 0
}
