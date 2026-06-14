// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"

	"oblikovati.org/app"
)

// identityCells is the 16-cell row-major identity transform, the flat-array JSON form a
// types.Matrix tool argument takes (drop at the assembly origin).
var identityCells = []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

// TestEndToEndPlaceComponent drives the assembly occurrence surface end to end over MCP: create
// an assembly and a part, place the part into the assembly, read the occurrence tree back, then
// remove the occurrence — proving the new assembly tools forward through the router to the live
// model (the loop an MCP client uses to build and verify assembly structure).
func TestEndToEndPlaceComponent(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	var asm, widget wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "widget.obk"}, &widget)

	// create_document activates the part it just made; the assembly must be active to place into.
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var placed wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{
		"document": widget.ID, "name": "widget:1", "transform": identityCells,
	}, &placed)
	if placed.Occurrence.Name != "widget:1" {
		t.Fatalf("placed occurrence name = %q, want %q", placed.Occurrence.Name, "widget:1")
	}

	var tree wire.OccurrencesResult
	callJSON(t, cs, "list_occurrences", nil, &tree)
	if len(tree.Occurrences) != 1 {
		t.Fatalf("list_occurrences = %d occurrences, want 1", len(tree.Occurrences))
	}
	if tree.Occurrences[0].ID != placed.Occurrence.ID {
		t.Errorf("listed occurrence id = %d, want the placed id %d", tree.Occurrences[0].ID, placed.Occurrence.ID)
	}

	var after wire.OccurrencesResult
	callJSON(t, cs, "remove_occurrence", map[string]any{"id": placed.Occurrence.ID}, &after)
	if len(after.Occurrences) != 0 {
		t.Errorf("after remove_occurrence: %d occurrences remain, want 0", len(after.Occurrences))
	}
}

// TestEndToEndPlaceComponentCopyAndState drives place_component_copy and the occurrence-state
// tools: a second instance reuses the first's component, and ground/suppress flip the flags the
// occurrence tree reports.
func TestEndToEndPlaceComponentCopyAndState(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	var asm, widget wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "widget.obk"}, &widget)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var first wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{
		"document": widget.ID, "name": "widget:1", "transform": identityCells,
	}, &first)

	var second wire.OccurrenceResult
	callJSON(t, cs, "place_component_copy", map[string]any{
		"source": first.Occurrence.ID, "name": "widget:2", "transform": identityCells,
	}, &second)

	var ground wire.OccurrenceResult
	callJSON(t, cs, "ground_occurrence", map[string]any{"id": first.Occurrence.ID, "grounded": true}, &ground)
	if !ground.Occurrence.Grounded {
		t.Errorf("ground_occurrence did not set grounded on occurrence %d", first.Occurrence.ID)
	}

	var suppress wire.OccurrenceResult
	callJSON(t, cs, "suppress_occurrence", map[string]any{"id": second.Occurrence.ID, "suppressed": true}, &suppress)
	if !suppress.Occurrence.Suppressed {
		t.Errorf("suppress_occurrence did not set suppressed on occurrence %d", second.Occurrence.ID)
	}

	var tree wire.OccurrencesResult
	callJSON(t, cs, "list_occurrences", nil, &tree)
	if len(tree.Occurrences) != 2 {
		t.Fatalf("list_occurrences = %d occurrences, want 2 (the copy reuses the definition)", len(tree.Occurrences))
	}
}
