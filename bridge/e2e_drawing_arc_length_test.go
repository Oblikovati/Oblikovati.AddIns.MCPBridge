// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	stdmath "math"
	"testing"
)

// TestEndToEndDrawingArcLength drives the arc-length dimension surface over MCP: a TOP view of a
// 2 cm-radius cylinder gets an arc-length dimension on its rim, re-measuring the true circumference
// (2π·20 ≈ 125.66 mm) through the live router→model→kernel stack (M14-F03 PBI-141, #388).
func TestEndToEndDrawingArcLength(t *testing.T) {
	cs := e2eClient(t, drawingCylinderSession(t)) // 2 cm-radius cylinder

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "TOP", "orientation": "top", "scale": 1.0, "centerXmm": 100.0, "centerYmm": 100.0,
	}, nil)

	var dim struct {
		Dimension struct {
			Type    string  `json:"type"`
			ValueMM float64 `json:"valueMm"`
		} `json:"dimension"`
	}
	callJSON(t, cs, "drawing_add_arc_length_dimension", map[string]any{
		"viewName": "TOP", "pickXmm": 100.0, "pickYmm": 100.0,
	}, &dim)

	if dim.Dimension.Type != "arcLength" {
		t.Errorf("dimension type = %q, want arcLength", dim.Dimension.Type)
	}
	if stdmath.Abs(dim.Dimension.ValueMM-2*stdmath.Pi*20) > 1e-3 {
		t.Errorf("arc-length = %v mm, want circumference 2π·20 ≈ 125.66", dim.Dimension.ValueMM)
	}
}
