// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	stdmath "math"
	"testing"
)

// TestEndToEndDrawingOrdinateDimensions drives the ordinate dimension surface over MCP: a FRONT
// view of a box gets a horizontal ordinate set from its bottom-left datum corner, producing one
// associative leader-to-value dimension per corner through the live router→model→kernel stack
// (M14-F03 PBI-141, #388).
func TestEndToEndDrawingOrdinateDimensions(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t)) // box 4×6×5 cm

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 1.0, "centerXmm": 100.0, "centerYmm": 100.0,
	}, nil)

	// FRONT (X right, Z up) at scale 1: corners at x∈{80,120}, y∈{75,125}. A horizontal ordinate set
	// from the bottom-left datum measures each corner's view-X offset.
	var set struct {
		Dimensions []struct {
			Type    string  `json:"type"`
			ValueMM float64 `json:"valueMm"`
		} `json:"dimensions"`
	}
	callJSON(t, cs, "drawing_add_ordinate_dimensions", map[string]any{
		"viewName": "FRONT", "axis": "horizontal",
		"datum":  []float64{80, 75},
		"points": [][]float64{{80, 75}, {120, 75}},
	}, &set)

	if len(set.Dimensions) != 2 {
		t.Fatalf("ordinate set = %d dimensions, want 2", len(set.Dimensions))
	}
	if set.Dimensions[0].Type != "ordinate" {
		t.Errorf("ordinate[0].Type = %q, want ordinate", set.Dimensions[0].Type)
	}
	// datum→itself = 0, datum→bottom-right = 4 cm width.
	if stdmath.Abs(set.Dimensions[0].ValueMM-0) > 1e-6 || stdmath.Abs(set.Dimensions[1].ValueMM-40) > 1e-6 {
		t.Errorf("ordinate values = %v / %v mm, want 0 / 40", set.Dimensions[0].ValueMM, set.Dimensions[1].ValueMM)
	}
}
