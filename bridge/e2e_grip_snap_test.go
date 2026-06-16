// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"oblikovati.org/api/wire"

	"oblikovati.org/app"
)

// TestEndToEndGripSnap drives the full Grip Snap surface (M12 #794) over MCP: build a box part, place
// two instances in an assembly, ground one, then GRIP-SNAP the free instance's bottom face onto the
// grounded top face WITHOUT naming a constraint. The host must infer a mate (two opposed planar faces),
// stack the free instance one box-height up, and report it — proving snap → router → inference → solver
// → live model. A prefer override then forces a flush instead.
func TestEndToEndGripSnap(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	var widget wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "widget.obk"}, &widget)
	topKey, botKey := boxFaceKeysByZ(t, cs)

	var asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var first, second wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{"document": widget.ID, "name": "box:1", "transform": identityCells}, &first)
	callJSON(t, cs, "place_component_copy", map[string]any{"source": first.Occurrence.ID, "name": "box:2", "transform": identityCells}, &second)
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": first.Occurrence.ID, "grounded": true}, nil)

	// Snap box:2's bottom face onto box:1's top face — no constraint named, the host infers it.
	var snapped wire.ConstraintResult
	callJSON(t, cs, "assembly_snap_constrain", map[string]any{
		"a": map[string]any{"occurrence": first.Occurrence.ID, "entity": topKey},
		"b": map[string]any{"occurrence": second.Occurrence.ID, "entity": botKey},
	}, &snapped)
	if snapped.Constraint.Type != "mate" {
		t.Fatalf("grip snap inferred %q, want mate (two opposed planar faces)", snapped.Constraint.Type)
	}

	var tree wire.OccurrencesResult
	callJSON(t, cs, "list_occurrences", nil, &tree)
	if z := occurrenceZ(t, tree, second.Occurrence.ID); math.Abs(z-2.0) > 1e-4 {
		t.Errorf("snapped instance Z = %v cm, want ~2.0 (bottom face on the grounded top face)", z)
	}

	// A prefer override forces the kind: snapping the same faces as a flush creates a flush.
	var flush wire.ConstraintResult
	callJSON(t, cs, "assembly_snap_constrain", map[string]any{
		"a":      map[string]any{"occurrence": first.Occurrence.ID, "entity": topKey},
		"b":      map[string]any{"occurrence": second.Occurrence.ID, "entity": botKey},
		"prefer": "flush",
	}, &flush)
	if flush.Constraint.Type != "flush" {
		t.Errorf("prefer=flush inferred %q, want flush", flush.Constraint.Type)
	}
}
