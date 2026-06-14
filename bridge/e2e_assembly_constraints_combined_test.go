// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"

	"oblikovati.org/app"
)

// This file validates COMBINED constraints between two parts over the bridge: several
// relationships stacked on the same pair, each removing further degrees of freedom, the way
// a real assembly is built up. The headline is the classic "cylindrical + angle" — an insert
// (collinear axes) leaves one spin DOF, and an angle between a face on each part restrains
// that spin to fully constrain the pair.

// boredBoxPart builds (in a fresh part document) a 40×30×20 mm box with an 8 mm bore drilled
// down its top face — an analytic cylinder, axis +Z — and returns the document id, the box's
// outer planar face keys, and the bore's cylindrical face key.
func boredBoxPart(t *testing.T, cs *mcp.ClientSession, name string) (docID uint64, k boxKeys, bore string) {
	t.Helper()
	var doc wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": name}, &doc)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"},
	}, nil)
	if healthy, reason := applyFeature(t, cs, "hole", map[string]any{"faceRef": topFaceKey(t, cs), "diameter": "8 mm", "depth": "20 mm"}); !healthy {
		t.Fatalf("bore unhealthy: %s", reason)
	}
	k = boxKeysFromTopology(t, cs)
	bore = cylinderFaceKey(t, cs)
	if bore == "" {
		t.Fatal("no cylindrical bore face found")
	}
	return doc.ID, k, bore
}

// zRotatedCells returns the 16-cell row-major transform of a rotation by angle about +Z plus
// a translation — used to start the free part spun off-axis so the angle constraint has a
// non-singular start.
func zRotatedCells(angle float64, tx, ty, tz float64) []float64 {
	c, s := math.Cos(angle), math.Sin(angle)
	return []float64{c, -s, 0, tx, s, c, 0, ty, 0, 0, 1, tz, 0, 0, 0, 1}
}

// TestEndToEndInsertPlusAngleRestrainsSpin is the canonical combined case: insert two bored
// parts so their bore axes are collinear (one spin DOF remains), then add an angle between a
// flat face on each to restrain the spin — fully constraining the free part (DOF 6 → 1 → 0).
func TestEndToEndInsertPlusAngleRestrainsSpin(t *testing.T) {
	cs := e2eClient(t, app.NewSession())
	part, k, bore := boredBoxPart(t, cs, "bored.obk")

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var g, f wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{"document": part, "name": "g:1", "transform": identityCells}, &g)
	// Place the free part spun 45° about Z and offset, so insert must re-seat it and the
	// angle has a non-singular (non-parallel) start.
	callJSON(t, cs, "place_component_copy", map[string]any{
		"source": g.Occurrence.ID, "name": "f:1", "transform": zRotatedCells(math.Pi/4, 3, 2, 5),
	}, &f)
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": g.Occurrence.ID, "grounded": true}, nil)

	// Insert the two bores: collinear axes + seated. One spin DOF remains.
	var insert wire.ConstraintResult
	callJSON(t, cs, "add_insert_constraint", map[string]any{
		"a": geomRef(g.Occurrence.ID, bore), "b": geomRef(f.Occurrence.ID, bore),
	}, &insert)
	var afterInsert wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &afterInsert)
	if dofOf(afterInsert, f.Occurrence.ID) != 1 || afterInsert.Status != "under-constrained" {
		t.Fatalf("after insert: free DOF = %d, status = %q, want 1 / under-constrained (%+v)",
			dofOf(afterInsert, f.Occurrence.ID), afterInsert.Status, afterInsert)
	}

	// Angle between the two +X faces (45° apart from the spun placement) restrains the spin.
	var angle wire.ConstraintResult
	callJSON(t, cs, "add_angle_constraint", map[string]any{
		"a": geomRef(g.Occurrence.ID, k.posX), "b": geomRef(f.Occurrence.ID, k.posX), "angle": math.Pi / 4,
	}, &angle)
	var afterAngle wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &afterAngle)
	if dofOf(afterAngle, f.Occurrence.ID) != 0 || afterAngle.Status != "well-constrained" {
		t.Errorf("after angle: free DOF = %d, status = %q, want 0 / well-constrained (%+v)",
			dofOf(afterAngle, f.Occurrence.ID), afterAngle.Status, afterAngle)
	}
	if afterAngle.DegreesOfFreedom != 0 {
		t.Errorf("assembly total DOF = %d, want 0 (fully constrained)", afterAngle.DegreesOfFreedom)
	}

	var list wire.ConstraintsResult
	callJSON(t, cs, "list_assembly_constraints", nil, &list)
	if len(list.Constraints) != 2 {
		t.Errorf("constraint set = %d, want 2 (insert + angle)", len(list.Constraints))
	}
}

// TestEndToEndStackedMatesProgressivelyConstrain stacks planar mates on perpendicular faces of
// two boxes and checks each one removes further DOF — the "seat a block into a corner" build-up
// (DOF 6 → 3 → 1).
func TestEndToEndStackedMatesProgressivelyConstrain(t *testing.T) {
	cs, g, f, k := twoBoxes(t)

	var mate1 wire.ConstraintResult
	callJSON(t, cs, "add_mate_constraint", map[string]any{
		"a": geomRef(g, k.topZ), "b": geomRef(f, k.topZ), "solution": "opposed",
	}, &mate1)
	var afterMate1 wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &afterMate1)
	if got := dofOf(afterMate1, f); got != 3 {
		t.Fatalf("after first mate: free DOF = %d, want 3", got)
	}

	// A flush on the +X faces aligns them coplanar — removing the spin and one in-plane slide.
	var flush wire.ConstraintResult
	callJSON(t, cs, "add_flush_constraint", map[string]any{
		"a": geomRef(g, k.posX), "b": geomRef(f, k.posX),
	}, &flush)
	var afterFlush wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &afterFlush)
	if got := dofOf(afterFlush, f); got != 1 {
		t.Errorf("after adding flush: free DOF = %d, want 1 (spin + one slide removed, one slide left)", got)
	}
	if afterFlush.Constraints != 2 {
		t.Errorf("active constraints = %d, want 2", afterFlush.Constraints)
	}
}
