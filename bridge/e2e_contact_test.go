// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/api/wire"
)

// TestEndToEndInterference runs a static interference analysis over two fully overlapping
// boxes and checks the overlap volume comes back (M12-F05, #362/#368). twoBoxes places both
// 40×30×20 mm boxes at identity, so they overlap entirely (24 cm³).
func TestEndToEndInterference(t *testing.T) {
	cs, _, _, _ := twoBoxes(t)

	var res wire.InterferenceResultsResult
	callJSON(t, cs, "analyze_interference", map[string]any{}, &res)
	if len(res.Results) != 1 {
		t.Fatalf("interference results = %d, want 1 overlapping pair: %+v", len(res.Results), res)
	}
	if res.Results[0].Volume <= 0 || res.TotalVolume <= 0 {
		t.Errorf("interference volume = %v (total %v), want > 0", res.Results[0].Volume, res.TotalVolume)
	}
}

// TestEndToEndContactSets builds a contact set with both occurrences and enables the contact
// solver over MCP (M12-F05).
func TestEndToEndContactSets(t *testing.T) {
	cs, grounded, free, _ := twoBoxes(t)

	var set wire.ContactSetResult
	callJSON(t, cs, "create_contact_set", map[string]any{"name": "drag"}, &set)
	callJSON(t, cs, "add_contact_member", map[string]any{"set": set.ContactSet.ID, "occurrence": grounded}, &wire.ContactSetResult{})

	var withBoth wire.ContactSetResult
	callJSON(t, cs, "add_contact_member", map[string]any{"set": set.ContactSet.ID, "occurrence": free}, &withBoth)
	if len(withBoth.ContactSet.Members) != 2 {
		t.Fatalf("contact set members = %v, want 2", withBoth.ContactSet.Members)
	}

	var solver wire.ContactSolverResult
	callJSON(t, cs, "set_contact_solver", map[string]any{"enabled": true}, &solver)
	if !solver.Solver.Enabled {
		t.Errorf("contact solver not enabled: %+v", solver.Solver)
	}
}
