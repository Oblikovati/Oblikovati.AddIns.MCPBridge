// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingNotesAndTables drives the note & custom-table surface over MCP: a leader note
// and a custom table both produce annotations through the live router→model stack (M14-F04
// PBI-144, #391).
func TestEndToEndDrawingNotesAndTables(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)

	var n struct {
		Annotation struct {
			Kind       string `json:"kind"`
			Tag        string `json:"tag"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_note", map[string]any{
		"name": "N", "xmm": 100.0, "ymm": 100.0, "text": "DEBURR", "leaderXmm": 140.0, "leaderYmm": 130.0,
	}, &n)
	if n.Annotation.Kind != "drawingNote" || n.Annotation.CurveCount == 0 || n.Annotation.Tag != "DEBURR" {
		t.Fatalf("note = %+v, want a drawingNote (text+leader) tagged DEBURR", n.Annotation)
	}

	var ct struct {
		Annotation struct {
			Kind     string `json:"kind"`
			RowCount int    `json:"rowCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_custom_table", map[string]any{
		"name": "CT", "xmm": 250.0, "ymm": 60.0,
		"headers": []string{"PARAM", "VALUE"},
		"rows":    [][]string{{"width", "60 mm"}, {"height", "40 mm"}},
	}, &ct)
	if ct.Annotation.Kind != "customTable" || ct.Annotation.RowCount != 2 {
		t.Fatalf("custom table = %+v, want a customTable with 2 rows", ct.Annotation)
	}
}
