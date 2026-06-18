// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingBalloon drives the balloon surface over MCP: a balloon with a leader is placed
// on a sheet, producing a circle + leader annotation carrying its item number through the live
// router→model→kernel stack (M14-F04 PBI-143, #390).
func TestEndToEndDrawingBalloon(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)

	var b struct {
		Annotation struct {
			Kind       string `json:"kind"`
			Tag        string `json:"tag"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_balloon", map[string]any{
		"name": "B", "xmm": 100.0, "ymm": 200.0, "item": 3, "leaderXmm": 120.0, "leaderYmm": 180.0,
	}, &b)

	if b.Annotation.Kind != "balloon" || b.Annotation.Tag != "3" {
		t.Errorf("balloon = %+v, want a balloon tagged item 3", b.Annotation)
	}
	if b.Annotation.CurveCount == 0 {
		t.Errorf("balloon = %d curves, want a circle + leader", b.Annotation.CurveCount)
	}
}
