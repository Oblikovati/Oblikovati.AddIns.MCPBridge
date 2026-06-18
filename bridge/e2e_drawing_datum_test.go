// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingDatumFeature drives the GD&T datum surface over MCP: a datum feature symbol
// (the letter A in a box with a datum triangle) is placed on a sheet, producing a framed annotation
// with box + triangle geometry through the live router→model→kernel stack (M14-F03 PBI-142, #389).
func TestEndToEndDrawingDatumFeature(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)

	var dat struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_datum_feature", map[string]any{
		"name": "DAT", "xmm": 70.0, "ymm": 70.0, "letter": "A",
	}, &dat)

	if dat.Annotation.Kind != "datumFeature" {
		t.Errorf("annotation kind = %q, want datumFeature", dat.Annotation.Kind)
	}
	if dat.Annotation.CurveCount == 0 {
		t.Errorf("datum = %d curves, want box + triangle geometry", dat.Annotation.CurveCount)
	}
}
