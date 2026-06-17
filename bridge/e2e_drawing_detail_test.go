// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestEndToEndDrawingDetailView drives the detail-view surface over MCP: create a drawing of a
// boxed part, add a base view, then magnify a circular region of it as a detail view and read
// the result back — proving the clip-and-rescale detail path through the live router→model→kernel
// stack (M14-F02 PBI-140, #387).
func TestEndToEndDrawingDetailView(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	// A large boundary around the front view's centre, so the magnified region keeps geometry.
	var det struct {
		View struct {
			Name      string  `json:"name"`
			Type      string  `json:"type"`
			BaseView  string  `json:"baseView"`
			Scale     float64 `json:"scale"`
			Projected bool    `json:"projected"`
		} `json:"view"`
	}
	callJSON(t, cs, "drawing_add_detail_view", map[string]any{
		"name": "DETAIL-A", "parentView": "FRONT", "boundaryXmm": 120.0, "boundaryYmm": 100.0,
		"radiusMm": 60.0, "scale": 4.0, "centerXmm": 120.0, "centerYmm": 240.0,
	}, &det)
	if det.View.Type != "detail" || det.View.BaseView != "FRONT" || det.View.Projected {
		t.Fatalf("detail view = %+v, want a detail off FRONT", det.View)
	}
	if det.View.Scale != 4.0 {
		t.Errorf("detail scale = %g, want 4", det.View.Scale)
	}

	var curves struct {
		Segments []struct {
			EdgeKey string `json:"edgeKey"`
		} `json:"segments"`
	}
	callJSON(t, cs, "drawing_view_curves", map[string]any{"view": "DETAIL-A"}, &curves)
	if len(curves.Segments) == 0 {
		t.Fatal("detail view returned no curves — clip kept nothing")
	}
}
