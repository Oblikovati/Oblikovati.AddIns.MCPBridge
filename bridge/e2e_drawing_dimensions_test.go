// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestEndToEndDrawingDimensions drives the linear-dimension surface over MCP: a drawing of a boxed
// part gets a base view, then a horizontal dimension across it, which is listed and re-measures
// the box's true width — exercising the dimension subsystem (snap-to-vertex, associative measure,
// glyph) through the live router→model→kernel stack (M14-F03 PBI-141, #388).
func TestEndToEndDrawingDimensions(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t)) // box 4×6×5 cm

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	// FRONT (X right, Z up) at scale 2: the 4 cm width spans x∈[80,160], the 5 cm height y∈[50,150].
	// A horizontal dimension across the bottom measures the true 40 mm width.
	var dim struct {
		Dimension struct {
			Type       string  `json:"type"`
			ViewName   string  `json:"viewName"`
			ValueMM    float64 `json:"valueMm"`
			Text       string  `json:"text"`
			CurveCount int     `json:"curveCount"`
		} `json:"dimension"`
	}
	callJSON(t, cs, "drawing_add_linear_dimension", map[string]any{
		"name": "W", "viewName": "FRONT", "type": "horizontal",
		"x1": 80.0, "y1": 50.0, "x2": 160.0, "y2": 50.0, "offsetMm": -12.0,
	}, &dim)

	if dim.Dimension.ViewName != "FRONT" || dim.Dimension.Type != "horizontal" {
		t.Fatalf("dimension = %+v, want a horizontal dim on FRONT", dim.Dimension)
	}
	if math.Abs(dim.Dimension.ValueMM-40) > 1e-6 {
		t.Errorf("measured value = %v mm, want 40 (the box's 4 cm width)", dim.Dimension.ValueMM)
	}
	if dim.Dimension.CurveCount == 0 || dim.Dimension.Text == "" {
		t.Errorf("dimension = %+v, want glyph curves + value text", dim.Dimension)
	}

	var list struct {
		Dimensions []struct {
			Name string `json:"name"`
		} `json:"dimensions"`
	}
	callJSON(t, cs, "drawing_list_dimensions", map[string]any{}, &list)
	if len(list.Dimensions) != 1 || list.Dimensions[0].Name != "W" {
		t.Fatalf("dimensions = %+v, want one named W", list.Dimensions)
	}
}
