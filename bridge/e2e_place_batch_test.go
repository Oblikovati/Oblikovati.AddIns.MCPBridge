// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
	"oblikovati.org/app"
)

// translateCells is a row-major translation matrix as the flat 16-cell array the tool accepts.
func translateCells(x float64) []float64 {
	return []float64{1, 0, 0, x, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

// TestE2EPlaceComponentCopiesBatch drives the batch placement tool end-to-end through the bridge:
// one call places several copies of a component and returns them all, with each nested transform
// round-tripping (the []float64 → types.Matrix marshaling), and the assembly ends with every copy.
// This is the large-assembly fast path — one call/one recompute instead of one per placement.
func TestE2EPlaceComponentCopiesBatch(t *testing.T) {
	cs := e2eClient(t, app.NewSession())
	var box, asm wire.DocumentInfo
	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "box.obk"}, &box)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "20 mm", "height": "20 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{"kind": "extrude", "args": map[string]any{
		"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}}, nil)
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)

	var first wire.OccurrenceResult
	callJSON(t, cs, "place_component", map[string]any{"document": box.ID, "name": "box:1", "transform": identityCells}, &first)

	var batch wire.PlaceByDefinitionBatchResult
	callJSON(t, cs, "place_component_copies", map[string]any{
		"source": first.Occurrence.ID,
		"placements": []map[string]any{
			{"name": "box:2", "transform": translateCells(3)},
			{"name": "box:3", "transform": translateCells(6)},
			{"name": "box:4", "transform": translateCells(9)},
		},
	}, &batch)

	if len(batch.Occurrences) != 3 {
		t.Fatalf("batch returned %d occurrences, want 3", len(batch.Occurrences))
	}
	if got := batch.Occurrences[1].Transform.Cells[3]; got != 6 { // box:3 at x=6
		t.Errorf("batched transform did not round-trip: box:3 x = %g, want 6", got)
	}
	assertOccurrenceCount(t, cs, 4) // box:1 + the three batched
}

// assertOccurrenceCount checks the active assembly's top-level occurrence count.
func assertOccurrenceCount(t *testing.T, cs *mcp.ClientSession, want int) {
	t.Helper()
	var occ wire.OccurrencesResult
	callJSON(t, cs, "list_occurrences", map[string]any{}, &occ)
	if len(occ.Occurrences) != want {
		t.Fatalf("assembly has %d occurrences, want %d", len(occ.Occurrences), want)
	}
}
