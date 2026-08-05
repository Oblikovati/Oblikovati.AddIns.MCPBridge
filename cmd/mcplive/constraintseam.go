// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"time"
)

// runConstraintSeam is the live check for Oblikovati#1625 (audit I2): sketch
// constraints now serialize through a paired codec registry and name their kind
// and refs over the API via the KindedConstraint capability — no consumer type
// switch, no default: save-time failure. It creates every wire-creatable 2D and
// 3D geometric constraint kind, round-trips the document through a real .obk
// save → force-close → reopen, and requires the enumerated constraint-kind
// census to survive identically. The 3D "equal" constraint is the regression
// centerpiece: before #1625 it was creatable and enumerable but registered in
// NEITHER serialize switch, so this exact save failed at runtime.
func runConstraintSeam(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addSeam2DConstraints(c)
	addSeam3DConstraints(c)
	before2D, before3D := constraintKindCensus(c, "list_sketch_constraints"), constraintKindCensus(c, "list_sketch3d_constraints")
	if c.err != nil {
		return c.err
	}
	if err := roundTripConstraintDocument(c); err != nil {
		return err
	}
	after2D, after3D := constraintKindCensus(c, "list_sketch_constraints"), constraintKindCensus(c, "list_sketch3d_constraints")
	if c.err != nil {
		return c.err
	}
	if err := requireSameCensus("2D constraint", before2D, after2D); err != nil {
		return err
	}
	if err := requireSameCensus("3D constraint", before3D, after3D); err != nil {
		return err
	}
	c.json("execute_command", map[string]any{"id": "View.Home"}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-constraintseam.png"}, nil)
	return c.err
}

// addSeam2DConstraints places disjoint geometry and one constraint of every
// wire-creatable 2D kind on it (the textBox anchor is auto-created with its
// text box). Geometry is laid out consistent with each relation so the
// on-add solve stays healthy.
func addSeam2DConstraints(c *caller) {
	c.json("add_sketch_text", map[string]any{"sketchIndex": 0, "anchor": []float64{20, 20}, "text": "I2", "height": "5 mm"}, nil)
	h := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0}, {2, 0}}})
	v := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{4, 0}, {4, 2}}})
	p1 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 4}, {2, 4}}})
	p2 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 5}, {2, 5}}})
	c1 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{8, 0}}, "radius": "10 mm"})
	c2 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{12, 0}}, "radius": "10 mm"})
	if c.err != nil {
		return
	}
	c.conAt(0, "horizontal", h[1], h[2])
	c.conAt(0, "vertical", v[1], v[2])
	c.conAt(0, "coincident", h[2], v[1])
	c.conAt(0, "parallel", p1[0], p2[0])
	c.conAt(0, "equalLength", p1[0], p2[0])
	c.conAt(0, "offset", p1[0], p2[0])
	c.conAt(0, "concentric", c1[0], c2[0])
	c.conAt(0, "equalRadius", c1[0], c2[0])
	c.conAt(0, "fix", p1[1])
	c.conAt(0, "ground", p2[1])
	c.conAt(0, "patternLink", p1[2], p2[2])
	addSeam2DMixedConstraints(c)
}

// addSeam2DMixedConstraints covers the mixed-operand kinds on their own geometry.
func addSeam2DMixedConstraints(c *caller) {
	onLine := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 8}, {4, 8}}})
	probe := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{1, 8}}})
	mid := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{2, 8}}})
	circ := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{8, 8}}, "radius": "10 mm"})
	rim := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{9, 8}}})
	tanLine := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{7, 9}, {9, 9}}})
	col1 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 11}, {2, 11}}})
	col2 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{3, 11}, {5, 11}}})
	perp := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 11}, {0, 13}}})
	symA := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{7, 12}}})
	symB := c.ids(map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{11, 12}}})
	symAbout := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{9, 11}, {9, 13}}})
	smLine := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 15}, {2, 15}}})
	smSpline := c.ids(map[string]any{"sketchIndex": 0, "kind": "spline", "points": [][]float64{{2, 15}, {3, 15.6}, {4, 15}}})
	if c.err != nil {
		return
	}
	c.conAt(0, "pointOnLine", probe[0], onLine[0])
	c.conAt(0, "midpoint", mid[0], onLine[0])
	c.conAt(0, "pointOnCircle", rim[0], circ[0])
	c.conAt(0, "tangent", tanLine[0], circ[0])
	c.conAt(0, "collinear", col1[0], col2[0])
	c.conAt(0, "perpendicular", col1[0], perp[0])
	c.conAt(0, "symmetry", symA[0], symB[0], symAbout[0])
	c.conAt(0, "smooth", smLine[0], smSpline[0])
	c.json("add_sketch_constraint", map[string]any{
		"sketchIndex": 0, "kind": "custom", "entities": []uint64{symA[0], symB[0]},
		"clientId": "mcplive.seam", "name": "seam-tag",
	}, nil)
}

// addSeam3DConstraints creates a 3D sketch and one constraint of every
// wire-creatable 3D kind — including "equal", the #1625 save-failure regression.
func addSeam3DConstraints(c *caller) {
	c.json("create_sketch3d", map[string]any{"name": "seam3d"}, nil)
	pA := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0, 2}}})
	pB := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0, 2}}})
	pC := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0, 4}}})
	l1 := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0, 0}, {2, 0, 0}}})
	l2 := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 1, 0}, {2, 1, 0}}})
	l3 := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0, 0}, {0, 2, 0}}})
	c1 := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{6, 0, 0}}, "axis": []float64{0, 0, 1}, "radius": "8 mm"})
	c2 := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{10, 0, 0}}, "axis": []float64{0, 0, 1}, "radius": "8 mm"})
	if c.err != nil {
		return
	}
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "coincident", "entities": []uint64{pA, pB}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "collinear", "entities": []uint64{pA, pB, pC}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "concentric", "entities": []uint64{pA, pC}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "equal", "entities": []uint64{c1, c2}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "parallel", "entities": []uint64{l1, l2}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "perpendicular", "entities": []uint64{l1, l3}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "midpoint", "entities": []uint64{pC, l1}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "ground", "entities": []uint64{pA}}, nil)
	for _, kind := range []string{"parallelToXAxis", "parallelToXYPlane"} {
		c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": kind, "entities": []uint64{l1}}, nil)
	}
	addSeam3DCurveJoins(c)
}

// addSeam3DCurveJoins covers the curve-join kinds (tangent/smooth/splineFit/
// helical) plus bend, which materializes with its own constraint.
func addSeam3DCurveJoins(c *caller) {
	jl := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 5, 0}, {2, 5, 0}}})
	arc := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "arc", "points": [][]float64{{2, 6, 0}, {2, 5, 0}, {3, 6, 0}}})
	sp := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "spline", "points": [][]float64{{3, 6, 0}, {4, 7, 0.5}, {5, 6, 1}}})
	fit := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{3, 6, 0}}})
	helix := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "helical", "points": [][]float64{{6, 0, 0}}, "radius": "8 mm", "pitch": "5 mm", "revolutions": 3, "mode": "pitchRevolution"})
	circle := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{6, 0, 0}}, "axis": []float64{0, 0, 1}, "radius": "8 mm"})
	b1 := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 9, 0}, {2, 9, 0}}})
	b2 := ent3D(c, map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{2, 9, 0}, {2, 11, 0}}})
	if c.err != nil {
		return
	}
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "tangent", "entities": []uint64{jl, arc}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "smooth", "entities": []uint64{arc, sp}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "splineFitPoints", "entities": []uint64{sp, fit}}, nil)
	c.json("add_sketch3d_constraint", map[string]any{"sketchIndex": 0, "kind": "helical", "entities": []uint64{helix, circle}}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "bend", "lines": []uint64{b1, b2}, "radius": "3 mm"}, nil)
}

// ent3D adds one 3D entity and returns its session id.
func ent3D(c *caller, args map[string]any) uint64 {
	var res struct {
		EntityID uint64 `json:"entityId"`
	}
	c.json("add_sketch3d_entity", args, &res)
	return res.EntityID
}

// constraintKindCensus enumerates sketch 0's constraints through the given
// list tool and counts them by their self-reported kind — the KindedConstraint
// capability the router derives every row from (#1625).
func constraintKindCensus(c *caller, tool string) map[string]int {
	var res struct {
		Constraints []struct {
			Kind string `json:"kind"`
		} `json:"constraints"`
	}
	c.json(tool, map[string]any{"sketchIndex": 0}, &res)
	census := map[string]int{}
	for _, con := range res.Constraints {
		census[con.Kind]++
	}
	return census
}

// roundTripConstraintDocument saves to a fresh .obk, force-closes everything,
// and reopens — the full constraint codec encode/decode path. Before #1625 the
// SAVE itself failed here on the 3D equal constraint.
func roundTripConstraintDocument(c *caller) error {
	path := fmt.Sprintf("/tmp/oblikovati-constraintseam-%d.obk", time.Now().UnixNano()%1000000)
	c.json("documents_save_as", map[string]any{"document": c.doc.ID, "newFullDocumentName": path}, nil)
	c.json("close_all_documents", map[string]any{"force": true}, nil)
	var opened struct {
		ID uint64 `json:"id"`
	}
	c.json("documents_open", map[string]any{"fullDocumentName": path, "visible": true}, &opened)
	if c.err != nil {
		return c.err
	}
	c.json("activate_document", map[string]any{"id": opened.ID}, nil)
	fmt.Printf("  round-tripped %s (reopened as document %d)\n", path, opened.ID)
	return c.err
}
