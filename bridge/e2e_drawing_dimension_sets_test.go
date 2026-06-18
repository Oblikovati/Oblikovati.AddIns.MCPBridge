// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	stdmath "math"
	"testing"
)

// TestEndToEndDrawingDimensionSets drives the baseline/chain dimension-set surface over MCP: a
// FRONT view of a box gets a baseline set across its four corners, producing three associative
// linear dimensions through the live router→model→kernel stack (M14-F03 PBI-141, #388).
func TestEndToEndDrawingDimensionSets(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t)) // box 4×6×5 cm

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 1.0, "centerXmm": 100.0, "centerYmm": 100.0,
	}, nil)

	// FRONT (X right, Z up) at scale 1: corners at x∈{80,120}, y∈{75,125}. A baseline set from the
	// bottom-left corner measures (aligned) to each of the other three.
	var set struct {
		Dimensions []struct {
			Type    string  `json:"type"`
			ValueMM float64 `json:"valueMm"`
		} `json:"dimensions"`
	}
	callJSON(t, cs, "drawing_add_baseline_dimensions", map[string]any{
		"viewName": "FRONT", "type": "aligned",
		"points": [][]float64{{80, 75}, {120, 75}, {120, 125}, {80, 125}},
	}, &set)

	if len(set.Dimensions) != 3 {
		t.Fatalf("baseline set = %d dimensions, want 3", len(set.Dimensions))
	}
	if stdmath.Abs(set.Dimensions[0].ValueMM-40) > 1e-6 { // datum→bottom-right = 4 cm width
		t.Errorf("baseline[0] = %v mm, want 40", set.Dimensions[0].ValueMM)
	}
}
