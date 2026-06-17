// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestEndToEndDrawingExtraViews drives the slice, breakout and draft view types over MCP: a
// drawing of a boxed part gets a base view, then a slice (cut outline), a breakout (interior
// revealed in a region) and a model-less draft frame — exercising the three remaining F02 view
// types through the live router→model→kernel stack (M14-F02 #812).
func TestEndToEndDrawingExtraViews(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	type viewResp struct {
		View struct {
			Type     string `json:"type"`
			BaseView string `json:"baseView"`
		} `json:"view"`
	}
	var slice, breakout, draft viewResp
	callJSON(t, cs, "drawing_add_slice_view", map[string]any{
		"name": "SL", "parentView": "FRONT", "x1": 80.0, "y1": 100.0, "x2": 160.0, "y2": 100.0, "centerXmm": 120.0, "centerYmm": 240.0,
	}, &slice)
	if slice.View.Type != "slice" || slice.View.BaseView != "FRONT" {
		t.Fatalf("slice view = %+v, want a slice off FRONT", slice.View)
	}
	callJSON(t, cs, "drawing_add_breakout_view", map[string]any{
		"name": "BO", "parentView": "FRONT", "boundaryXmm": 120.0, "boundaryYmm": 100.0, "radiusMm": 60.0, "centerXmm": 300.0, "centerYmm": 100.0,
	}, &breakout)
	if breakout.View.Type != "breakout" || breakout.View.BaseView != "FRONT" {
		t.Fatalf("breakout view = %+v, want a breakout off FRONT", breakout.View)
	}
	callJSON(t, cs, "drawing_add_draft_view", map[string]any{
		"name": "DR", "widthMm": 80.0, "heightMm": 50.0, "centerXmm": 120.0, "centerYmm": 340.0,
	}, &draft)
	if draft.View.Type != "draft" {
		t.Fatalf("draft view = %+v, want a draft view", draft.View)
	}

	// The slice comes back as cut-outline curves; the draft as a 4-edge frame.
	var curves struct {
		Segments []struct {
			Kind string `json:"kind"`
		} `json:"segments"`
	}
	callJSON(t, cs, "drawing_view_curves", map[string]any{"view": "DR"}, &curves)
	if len(curves.Segments) != 4 {
		t.Errorf("draft view = %d curves, want a 4-edge frame", len(curves.Segments))
	}
}
