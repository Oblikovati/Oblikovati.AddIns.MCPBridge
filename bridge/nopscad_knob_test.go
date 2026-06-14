// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestNopKnobRoundedRim models a knob body (NopSCADlib printed/knob.scad) the Inventor way:
// a half-section whose outer top corner is a rounded fillet — a quarter-circle ARC — revolved
// 360° about the Y axis into a cylinder with a rounded top rim. It is the first part to revolve
// a profile containing an arc (a curved revolved face), and the rim's exact solid-of-revolution
// volume is the check. A parameter edit (taller knob) must rebuild and track the volume.
//
// (NopSCADlib's knob is a slightly domed truncated cone; this faithful simplification keeps the
// distinctive rounded-rim revolve while admitting an exact analytic volume.)
//
// Reference: NopSCADlib/printed/knob.scad (the rotate_extrude of a rounded_polygon body).
func TestNopKnobRoundedRim(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "knobD", "expression": "15 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "knobH", "expression": "18 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "rimR", "expression": "2 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	// Half-section (cm seeds; dimensions below drive it). R=0.75, H=1.8, r=0.2.
	// P0(0,0)→P1(.75,0)→P2(.75,1.6)→[arc rim, centre C(.55,1.6)]→P3(.55,1.8)→P4(0,1.8)→P0.
	mkLine := func(x0, y0, x1, y1 float64) []uint64 {
		return idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "line",
			"points": [][]float64{{x0, y0}, {x1, y1}}})
	}
	l0 := mkLine(0, 0, 0.75, 0)      // bottom
	l1 := mkLine(0.75, 0, 0.75, 1.6) // side
	l3 := mkLine(0.55, 1.8, 0, 1.8)  // top
	l4 := mkLine(0, 1.8, 0, 0)       // axis
	// Rim arc: centre C, start on the side (P2), end on the top (P3), sweeping CCW.
	arc := idsOf(t, cs, map[string]any{"sketchIndex": 0, "kind": "arc", "ccw": true,
		"points": [][]float64{{0.55, 1.6}, {0.75, 1.6}, {0.55, 1.8}}})
	cC, aStart, aEnd := arc[1], arc[2], arc[3] // idsOf prepends the entity id
	p := func(l []uint64) (uint64, uint64) { return l[1], l[2] }
	p0a, p0b := p(l0)
	p1a, p1b := p(l1)
	p3a, p3b := p(l3)
	p4a, p4b := p(l4)

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	// Chain into a closed loop: bottom → side → arc → top → axis → bottom.
	con("coincident", p0b, p1a)    // P1
	con("coincident", p1b, aStart) // P2 (side top = arc start)
	con("coincident", aEnd, p3a)   // P3 (arc end = top start)
	con("coincident", p3b, p4a)    // P4
	con("coincident", p4b, p0a)    // P0
	// Orientations.
	con("horizontal", p0a, p0b) // bottom
	con("vertical", p1a, p1b)   // side
	con("horizontal", p3a, p3b) // top
	con("vertical", p4a, p4b)   // axis (x = 0)
	con("ground", p0a)          // P0 at the origin
	// Pin the arc centre square to its tangent points so the rim is a clean quarter circle.
	con("horizontal", cC, aStart) // C level with P2
	con("vertical", cC, aEnd)     // C plumb with P3

	dim := func(expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": ents, "expression": expr}, nil)
	}
	dim("knobD/2", p0a, p0b)      // bottom radius R
	dim("knobH - rimR", p1a, p1b) // straight side height
	dim("rimR", cC, aStart)       // rim radius (horizontal leg)
	dim("rimR", cC, aEnd)         // rim radius (vertical leg)

	var solve struct {
		DOF int `json:"dof"`
	}
	callJSON(t, cs, "solve_sketch", map[string]any{"sketchIndex": 0}, &solve)
	if solve.DOF != 0 {
		t.Fatalf("knob section not fully constrained: dof=%d, want 0", solve.DOF)
	}

	prof := closedProfileIndex(t, cs)
	if healthy, reason := applyFeature(t, cs, "revolve", map[string]any{
		"sketchIndex": 0, "profileIndex": prof, "axisRef": "origin/axis/y", "angle": "360 deg",
	}); !healthy {
		t.Fatalf("revolve unhealthy: %s", reason)
	}

	// Exact solid-of-revolution volume: a cylinder R×(H−r) plus the rounded-rim cap, cm^3.
	// Cap = π[(R−r)²r + (π/2)(R−r)r² + (2/3)r³] (revolving the quarter-disc + its core).
	wantVol := func(dMM, hMM, rMM float64) float64 {
		R, H, r := dMM/20, hMM/10, rMM/10
		rimCap := math.Pi * ((R-r)*(R-r)*r + (math.Pi/2)*(R-r)*r*r + (2.0/3.0)*r*r*r)
		return math.Pi*R*R*(H-r) + rimCap
	}
	if got, w := partVolume(t, cs), wantVol(15, 18, 2); math.Abs(got-w)/w > 0.02 {
		t.Errorf("knob volume = %.6f cm^3, want ~%.6f", got, w)
	}
	// Parametric: a taller knob adds a cylinder slice; the rim cap is unchanged.
	callJSON(t, cs, "set_parameter", map[string]any{"name": "knobH", "expression": "24 mm"}, nil)
	if got, w := partVolume(t, cs), wantVol(15, 24, 2); math.Abs(got-w)/w > 0.02 {
		t.Errorf("taller knob volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
