// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// TestEndToEndFlexibleChild places a sub-assembly into a top assembly, marks it flexible, and
// positions its child independently over MCP — the F06 independent-solve path including the
// flat-matrix mcp:input override for set_flexible_child (M12-F06, #822).
func TestEndToEndFlexibleChild(t *testing.T) {
	cs := e2eClient(t, app.NewSession())

	var part, sub, top wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "leaf.obk"}, &part)
	readBoxKeys(t, cs) // build a body in the part so it has placeable geometry

	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "sub.obk"}, &sub)
	callJSON(t, cs, "activate_document", map[string]any{"id": sub.ID}, nil)
	callJSON(t, cs, "place_component", map[string]any{"document": part.ID, "name": "p:1", "transform": identityCells}, &wire.OccurrenceResult{})

	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "top.obk"}, &top)
	callJSON(t, cs, "activate_document", map[string]any{"id": top.ID}, nil)
	var placed wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{"document": sub.ID, "name": "sub:1", "transform": identityCells}, &placed)

	var flexed wire.OccurrenceResult
	callJSON(t, cs, "set_flexible_occurrence", map[string]any{"id": placed.Occurrence.ID, "flexible": true}, &flexed)
	if !flexed.Occurrence.Flexible {
		t.Fatalf("occurrence not flexible after set_flexible_occurrence: %+v", flexed.Occurrence)
	}

	// Lift the child 40 mm along Z within this placement only; the flat 16-cell transform must
	// survive the mcp:input override (types.Matrix would otherwise reflect as an object).
	lift := []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 4, 0, 0, 0, 1}
	var moved wire.OccurrenceResult
	callJSON(t, cs, "set_flexible_child", map[string]any{
		"occurrence": placed.Occurrence.ID, "child": "p:1", "transform": lift,
	}, &moved)
	if moved.Occurrence.ID != placed.Occurrence.ID {
		t.Errorf("set_flexible_child returned occurrence %d, want %d", moved.Occurrence.ID, placed.Occurrence.ID)
	}
}
