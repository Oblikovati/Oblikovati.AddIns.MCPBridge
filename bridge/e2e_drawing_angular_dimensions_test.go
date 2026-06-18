// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	stdmath "math"
	"testing"
)

// TestEndToEndDrawingAngularDimension drives the angular-dimension surface over MCP: a FRONT view
// of a box gets an angular dimension between a horizontal and a vertical edge, re-deriving 90°
// (reported in valueDeg) through the live router→model→kernel stack (M14-F03 PBI-141, #388).
func TestEndToEndDrawingAngularDimension(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t)) // box 4×6×5 cm

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 1.0, "centerXmm": 100.0, "centerYmm": 100.0,
	}, nil)

	// FRONT (X right, Z up) at scale 1: bottom edge near y=75, side edges at x∈{80,120}. Pick a
	// horizontal and a vertical edge → a 90° corner.
	var dim struct {
		Dimension struct {
			Type       string  `json:"type"`
			ValueDeg   float64 `json:"valueDeg"`
			CurveCount int     `json:"curveCount"`
			Text       string  `json:"text"`
		} `json:"dimension"`
	}
	callJSON(t, cs, "drawing_add_angular_dimension", map[string]any{
		"name": "A1", "viewName": "FRONT", "x1": 100.0, "y1": 75.0, "x2": 120.0, "y2": 100.0,
	}, &dim)

	if dim.Dimension.Type != "angular" || stdmath.Abs(dim.Dimension.ValueDeg-90) > 1e-6 {
		t.Fatalf("angular dimension = %+v, want a 90° angle", dim.Dimension)
	}
	if dim.Dimension.CurveCount == 0 || dim.Dimension.Text == "" {
		t.Errorf("angular dimension = %+v, want arc glyph + text", dim.Dimension)
	}
}
