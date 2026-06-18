// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingHoleNotes drives the hole-note surface over MCP: a TOP view of a cylinder gets
// a hole note — one leadered diameter callout (its two coincident rims dedup) — through the live
// router→model→kernel stack (M14-F07, #637).
func TestEndToEndDrawingHoleNotes(t *testing.T) {
	cs := e2eClient(t, drawingCylinderSession(t)) // 2 cm-radius cylinder

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "TOP", "orientation": "top", "scale": 1.0, "centerXmm": 100.0, "centerYmm": 100.0,
	}, nil)

	var hn struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
			RowCount   int    `json:"rowCount"`
		} `json:"annotation"`
	}
	// Combined grouping flows over the wire; one cylinder rim → one callout.
	callJSON(t, cs, "drawing_add_hole_notes", map[string]any{"name": "HN", "viewName": "TOP", "quantity": "combined"}, &hn)

	if hn.Annotation.Kind != "holeNote" || hn.Annotation.CurveCount == 0 {
		t.Fatalf("hole notes = %+v, want a holeNote with a leadered callout", hn.Annotation)
	}
	if hn.Annotation.RowCount != 1 {
		t.Errorf("hole notes rowCount = %d, want 1 (the two coincident rims dedup)", hn.Annotation.RowCount)
	}
}
