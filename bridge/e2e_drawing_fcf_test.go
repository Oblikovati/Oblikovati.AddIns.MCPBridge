// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingFeatureControlFrame drives the GD&T surface over MCP: a position feature
// control frame with two datums is placed on a sheet, producing a framed annotation with frame +
// symbol geometry through the live router→model→kernel stack (M14-F03 PBI-142, #389).
func TestEndToEndDrawingFeatureControlFrame(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)

	var fcf struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_feature_control_frame", map[string]any{
		"name": "FCF", "xmm": 60.0, "ymm": 60.0,
		"characteristic": "position", "tolerance": "0.5", "datums": []string{"A", "B"},
	}, &fcf)

	if fcf.Annotation.Kind != "featureControlFrame" {
		t.Errorf("annotation kind = %q, want featureControlFrame", fcf.Annotation.Kind)
	}
	if fcf.Annotation.CurveCount == 0 {
		t.Errorf("FCF = %d curves, want frame + symbol geometry", fcf.Annotation.CurveCount)
	}
}
