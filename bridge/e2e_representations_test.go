// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
)

// TestEndToEndLODRepresentation captures a level-of-detail rep, suppresses a member in it,
// activates it, and checks the suppression reaches the live occurrence (M12-F04, #361/#367).
func TestEndToEndLODRepresentation(t *testing.T) {
	cs, _, free, _ := twoBoxes(t)

	var lod wire.LODResult
	callJSON(t, cs, "capture_lod", map[string]any{"name": "simplified"}, &lod)

	var suppressed wire.LODResult
	callJSON(t, cs, "set_lod_suppressed", map[string]any{
		"rep": lod.Representation.ID, "occurrence": free, "suppressed": true,
	}, &suppressed)
	if suppressed.Representation.SuppressedCount != 1 {
		t.Fatalf("rep suppressed count = %d, want 1", suppressed.Representation.SuppressedCount)
	}

	callJSON(t, cs, "activate_lod", map[string]any{"id": lod.Representation.ID}, &wire.LODResult{})

	var occs wire.OccurrencesResult
	callJSON(t, cs, "list_occurrences", map[string]any{}, &occs)
	if !occurrenceSuppressed(occs.Occurrences, free) {
		t.Errorf("occurrence %d not suppressed after activating LOD: %+v", free, occs.Occurrences)
	}
}

// TestEndToEndModelState captures a LOD and a model state that selects it, then activates the
// state — exercising the model-state layer that selects one rep per family (M12-F04).
func TestEndToEndModelState(t *testing.T) {
	cs, _, _, _ := twoBoxes(t)

	var lod wire.LODResult
	callJSON(t, cs, "capture_lod", map[string]any{"name": "lightweight"}, &lod)

	var ms wire.ModelStateResult
	callJSON(t, cs, "create_model_state", map[string]any{"name": "review", "levelOfDetail": lod.Representation.Name}, &ms)
	if ms.ModelState.LevelOfDetail != lod.Representation.Name {
		t.Fatalf("model state LOD = %q, want %q", ms.ModelState.LevelOfDetail, lod.Representation.Name)
	}

	var activated wire.ModelStateResult
	callJSON(t, cs, "activate_model_state", map[string]any{"id": ms.ModelState.ID}, &activated)
	if !activated.ModelState.Active {
		t.Errorf("model state not active after activate: %+v", activated.ModelState)
	}
}

// occurrenceSuppressed reports whether the occurrence with id target is marked suppressed in
// the tree (top level only — the boxes here are top-level placements).
func occurrenceSuppressed(tree []wire.OccurrenceInfo, target uint64) bool {
	for _, o := range tree {
		if o.ID == target {
			return o.Suppressed
		}
	}
	return false
}
