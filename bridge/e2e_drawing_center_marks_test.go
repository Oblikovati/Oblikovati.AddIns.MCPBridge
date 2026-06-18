// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingCenterMarks drives the centre-mark surface over MCP: a TOP view of a
// 2 cm-radius cylinder gets centre marks on its circular edges, producing one crosshair (the two
// coincident rims dedup) through the live router→model→kernel stack (M14-F03 PBI-142, #389).
func TestEndToEndDrawingCenterMarks(t *testing.T) {
	cs := e2eClient(t, drawingCylinderSession(t)) // 2 cm-radius cylinder

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "TOP", "orientation": "top", "scale": 1.0, "centerXmm": 100.0, "centerYmm": 100.0,
	}, nil)

	var marks struct {
		Annotations []struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotations"`
	}
	callJSON(t, cs, "drawing_add_center_marks", map[string]any{"viewName": "TOP"}, &marks)

	if len(marks.Annotations) != 1 {
		t.Fatalf("centre marks = %d, want 1 (the two coincident rims dedup)", len(marks.Annotations))
	}
	if marks.Annotations[0].Kind != "centerMark" || marks.Annotations[0].CurveCount == 0 {
		t.Errorf("centre mark = %+v, want a centerMark with a crosshair glyph", marks.Annotations[0])
	}
}
