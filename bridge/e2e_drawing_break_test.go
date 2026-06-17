// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestEndToEndDrawingBreakView drives the break-view surface over MCP: create a drawing of a
// boxed part, add a base view, then compress it with a break view and read the result back —
// proving the band-removal + break-glyph path through the live router→model→kernel stack
// (M14-F02 PBI-140, #387).
func TestEndToEndDrawingBreakView(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	var brk struct {
		View struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			BaseView  string `json:"baseView"`
			Projected bool   `json:"projected"`
		} `json:"view"`
	}
	callJSON(t, cs, "drawing_add_break_view", map[string]any{
		"name": "BREAK-A", "parentView": "FRONT", "orientation": "horizontal",
		"gapStartMm": 112.0, "gapEndMm": 128.0, "centerXmm": 120.0, "centerYmm": 240.0,
	}, &brk)
	if brk.View.Type != "break" || brk.View.BaseView != "FRONT" || brk.View.Projected {
		t.Fatalf("break view = %+v, want a break off FRONT", brk.View)
	}

	// The compressed view comes back with model edges plus break-line glyph curves.
	var curves struct {
		Segments []struct {
			Kind string `json:"kind"`
		} `json:"segments"`
	}
	callJSON(t, cs, "drawing_view_curves", map[string]any{"view": "BREAK-A"}, &curves)
	var glyphs int
	for _, s := range curves.Segments {
		if s.Kind == "break" {
			glyphs++
		}
	}
	if len(curves.Segments) == 0 {
		t.Fatal("break view returned no curves")
	}
	if glyphs == 0 {
		t.Error("break view returned no break-line glyph curves")
	}
}
