// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingHoleTable drives the hole-table surface over MCP: a TOP view of a
// 2 cm-radius cylinder gets a hole table listing its one distinct circular edge (the two
// coincident rims dedup), producing a one-row table with grid geometry through the live
// router→model→kernel stack (M14-F04 PBI-144, #391).
func TestEndToEndDrawingHoleTable(t *testing.T) {
	cs := e2eClient(t, drawingCylinderSession(t)) // 2 cm-radius cylinder

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "TOP", "orientation": "top", "scale": 1.0, "centerXmm": 100.0, "centerYmm": 100.0,
	}, nil)

	var ht struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
			RowCount   int    `json:"rowCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_hole_table", map[string]any{
		"name": "HT", "viewName": "TOP", "xmm": 220.0, "ymm": 240.0,
	}, &ht)

	if ht.Annotation.Kind != "holeTable" || ht.Annotation.CurveCount == 0 {
		t.Fatalf("hole table = %+v, want a holeTable with grid geometry", ht.Annotation)
	}
	if ht.Annotation.RowCount != 1 {
		t.Errorf("hole table rowCount = %d, want 1 (the two coincident rims dedup)", ht.Annotation.RowCount)
	}
}
