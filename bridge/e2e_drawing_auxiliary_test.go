// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestEndToEndDrawingAuxiliaryView drives the auxiliary-view surface over MCP: create a drawing
// of a boxed part, add a base view, then fold an auxiliary view off it and read its curves —
// proving the auxiliary projection (fold line → perpendicular projection) works through the live
// router→model→kernel stack (M14-F02 PBI-140, #387).
func TestEndToEndDrawingAuxiliaryView(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	var aux struct {
		View struct {
			Name         string  `json:"name"`
			Type         string  `json:"type"`
			BaseView     string  `json:"baseView"`
			Projected    bool    `json:"projected"`
			FoldAngleDeg float64 `json:"foldAngleDeg"`
			VisibleCount int     `json:"visibleCount"`
		} `json:"view"`
	}
	callJSON(t, cs, "drawing_add_auxiliary_view", map[string]any{
		"name": "AUX", "parentView": "FRONT", "foldAngleDeg": 30.0, "centerXmm": 120.0, "centerYmm": 240.0,
	}, &aux)
	if aux.View.Type != "auxiliary" || aux.View.BaseView != "FRONT" || aux.View.Projected {
		t.Fatalf("auxiliary view = %+v, want an auxiliary off FRONT", aux.View)
	}
	if aux.View.VisibleCount == 0 {
		t.Fatalf("auxiliary view returned no visible curves: %+v", aux.View)
	}

	// The curves come back keyed to model edges (associative), like any other view.
	var curves struct {
		Segments []struct {
			Visible bool   `json:"visible"`
			EdgeKey string `json:"edgeKey"`
		} `json:"segments"`
	}
	callJSON(t, cs, "drawing_view_curves", map[string]any{"view": "AUX"}, &curves)
	if len(curves.Segments) == 0 {
		t.Fatal("AUX view returned no drawing curves")
	}
	for _, seg := range curves.Segments {
		if seg.EdgeKey == "" {
			t.Errorf("auxiliary curve segment %+v carries no source edge key", seg)
		}
	}
}
