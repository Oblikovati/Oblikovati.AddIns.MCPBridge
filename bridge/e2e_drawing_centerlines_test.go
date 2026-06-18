// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingCenterlines drives the centerline surface over MCP: a FRONT view of a box gets
// its horizontal+vertical dash-dot symmetry centerlines through the live router→model→kernel stack
// (M14-F03 PBI-142, #389).
func TestEndToEndDrawingCenterlines(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t)) // box 4×6×5 cm

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	var cl struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_centerlines", map[string]any{"viewName": "FRONT"}, &cl)

	if cl.Annotation.Kind != "centerline" {
		t.Errorf("annotation kind = %q, want centerline", cl.Annotation.Kind)
	}
	if cl.Annotation.CurveCount < 4 {
		t.Errorf("centerlines = %d curves, want a dash-dot cross (many segments)", cl.Annotation.CurveCount)
	}
}
