// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// rectIDs adds a corner rectangle and returns its 4 member line ids
// (bottom,right,top,left) and 4 corner point ids (bl,br,tr,tl). The composite is
// built as closedLoop([bl,br,tr,tl]); both arrays follow that first-seen order.
func rectIDs(t *testing.T, cs *mcp.ClientSession, points [][]float64) (lines, corners []uint64) {
	t.Helper()
	var r struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "rectangle", "points": points}, &r)
	if len(r.EntityIDs) != 4 || len(r.PointIDs) != 4 {
		t.Fatalf("rectangle returned %d lines, %d points; want 4,4", len(r.EntityIDs), len(r.PointIDs))
	}
	return r.EntityIDs, r.PointIDs
}

// TestNopRectangularTube models NopSCADlib's rectangular_tube: a hollow rectangular
// section of wall thickness t, extruded to length L. The original rounds the corners
// (fillet=0.5); a faithful sharp-cornered CAD section is modelled here (the fillet is
// a cosmetic/strength detail that does not change the construction path), so the
// volume is the exact sharp-corner value (W·H − (W−2t)(H−2t))·L.
//
// Two concentric corner rectangles in one sketch form an annular profile (outer ring
// with a rectangular hole); it is fully constrained (0 DOF) with rectilinear edges,
// a grounded outer corner, the outer W/H, and the wall thickness on all four sides.
//
// Reference: NopSCADlib/utils/tube.scad
//
//	module rectangular_tube(size, thickness=1, fillet=0.5)
//	    extrude difference(rounded_square(size), rounded_square(size-2t));
func TestNopRectangularTube(t *testing.T) {
	cs := freshPart(t)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "wd", "expression": "20 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "ht", "expression": "15 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "thk", "expression": "1.5 mm"}, nil)
	callJSON(t, cs, "add_parameter", map[string]any{"name": "len", "expression": "30 mm"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)

	con := func(kind string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": ents}, nil)
	}
	dim := func(expr string, ents ...uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "distance", "entities": ents, "expression": expr}, nil)
	}

	offset := func(expr string, point, line uint64) {
		callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": 0, "kind": "offsetDim", "entities": []uint64{point, line}, "expression": expr}, nil)
	}

	// Outer rectangle, grounded at the origin. lines[0..3] = bottom,right,top,left.
	oLines, oPts := rectIDs(t, cs, [][]float64{{0, 0}, {2.0, 1.5}})
	obl, obr, otr, otl := oPts[0], oPts[1], oPts[2], oPts[3]
	con("horizontal", obl, obr)
	con("horizontal", otl, otr)
	con("vertical", obl, otl)
	con("vertical", obr, otr)
	con("ground", obl)
	dim("wd", obl, obr)
	dim("ht", obl, otl)

	// Inner rectangle: rigid (rectilinear) and sized to wd-2thk × ht-2thk, then pinned
	// inside the outer ring by perpendicular offsets of one corner to the two adjacent
	// outer walls — exactly the constant-wall-thickness inset.
	_, iPts := rectIDs(t, cs, [][]float64{{0.15, 0.15}, {1.85, 1.35}})
	ibl, ibr, itr, itl := iPts[0], iPts[1], iPts[2], iPts[3]
	con("horizontal", ibl, ibr)
	con("horizontal", itl, itr)
	con("vertical", ibl, itl)
	con("vertical", ibr, itr)
	dim("wd - 2 * thk", ibl, ibr)
	dim("ht - 2 * thk", ibl, itl)
	offset("thk", ibl, oLines[3]) // inner BL is thk from the outer left wall
	offset("thk", ibl, oLines[0]) // inner BL is thk from the outer bottom wall

	requireDOF(t, cs, 0)

	annulus := profileWithHole(t, cs)
	if annulus < 0 {
		t.Fatal("no annular profile (rectangular ring with rectangular hole) found")
	}
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{
		"sketchIndex": 0, "profileIndex": annulus, "distance": "len", "operation": "new",
	}); !healthy {
		t.Fatalf("rectangular_tube extrude unhealthy: %s", reason)
	}

	want := func(wMM, hMM, tMM, lMM float64) float64 {
		w, h, tt, l := wMM/10, hMM/10, tMM/10, lMM/10 // mm -> cm
		return (w*h - (w-2*tt)*(h-2*tt)) * l
	}
	if got, w := partVolume(t, cs), want(20, 15, 1.5, 30); math.Abs(got-w)/w > 0.02 {
		t.Errorf("rectangular_tube volume = %.6f cm^3, want ~%.6f", got, w)
	}
	callJSON(t, cs, "set_parameter", map[string]any{"name": "thk", "expression": "2 mm"}, nil)
	if got, w := partVolume(t, cs), want(20, 15, 2, 30); math.Abs(got-w)/w > 0.02 {
		t.Errorf("resized rectangular_tube volume = %.6f cm^3, want ~%.6f", got, w)
	}
}
