// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndModelHealth drives the model-health aggregation over MCP: a clean box part reports
// "ok" with nothing to repair, and after suppressing its extrude the feature is enumerated as
// suppressed — through the live router→model stack (M18-F02 #430).
func TestEndToEndModelHealth(t *testing.T) {
	cs := e2eClient(t, seededSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "box.opd"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "50 mm"},
	}, nil)

	var health struct {
		Overall   string `json:"overall"`
		SickCount int    `json:"sickCount"`
		Unhealthy []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"unhealthy"`
	}
	callJSON(t, cs, "analysis_model_health", map[string]any{}, &health)
	if health.Overall != "ok" || health.SickCount != 0 || len(health.Unhealthy) != 0 {
		t.Fatalf("clean part health = %+v, want ok with nothing unhealthy", health)
	}

	// Suppress the extrude (its id comes from the model tree), then it is enumerated as suppressed.
	var tree struct {
		Features []struct {
			ID uint64 `json:"id"`
		} `json:"features"`
	}
	callJSON(t, cs, "get_model_tree", map[string]any{}, &tree)
	if len(tree.Features) == 0 {
		t.Fatal("model tree has no features")
	}
	callJSON(t, cs, "features_set_suppressed", map[string]any{"id": tree.Features[0].ID, "suppressed": true}, nil)

	health.Unhealthy = nil
	callJSON(t, cs, "analysis_model_health", map[string]any{}, &health)
	if len(health.Unhealthy) != 1 || health.Unhealthy[0].Status != "suppressed" {
		t.Errorf("after suppress = %+v, want one suppressed feature", health)
	}
}
