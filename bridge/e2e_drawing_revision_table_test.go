// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingRevisionTable drives the revision-table & revision-tag surface over MCP: a
// two-row revision table and a revision tag both produce annotations through the live
// router→model stack (M14-F04 PBI-144, #391).
func TestEndToEndDrawingRevisionTable(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)

	var rt struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
			RowCount   int    `json:"rowCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_revision_table", map[string]any{
		"name": "RT", "xmm": 250.0, "ymm": 60.0,
		"rows": []map[string]any{
			{"revision": "A", "date": "2026-06-01", "description": "Initial release"},
			{"revision": "B", "date": "2026-06-18", "description": "Added holes"},
		},
	}, &rt)
	if rt.Annotation.Kind != "revisionTable" || rt.Annotation.CurveCount == 0 || rt.Annotation.RowCount != 2 {
		t.Fatalf("revision table = %+v, want a revisionTable with 2 rows + grid geometry", rt.Annotation)
	}

	var tag struct {
		Annotation struct {
			Kind string `json:"kind"`
			Tag  string `json:"tag"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_revision_tag", map[string]any{
		"name": "RT1", "xmm": 120.0, "ymm": 90.0, "revision": "B",
	}, &tag)
	if tag.Annotation.Kind != "revisionTag" || tag.Annotation.Tag != "B" {
		t.Fatalf("revision tag = %+v, want a revisionTag tagged B", tag.Annotation)
	}
}
