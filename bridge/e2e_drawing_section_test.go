// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestEndToEndDrawingSectionView drives the section-view surface over MCP: create a drawing of a
// boxed part, add a base view, then cut a section view through it and read its curves — proving
// the clipped-HLR cut-away (cut outline + hatch) works through the live router→model→kernel stack
// (M14-F02 PBI-140, #387).
func TestEndToEndDrawingSectionView(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	// A horizontal cut line straight across the FRONT view at its centre (sheet mm).
	var sec struct {
		View struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			BaseView  string `json:"baseView"`
			Projected bool   `json:"projected"`
		} `json:"view"`
	}
	callJSON(t, cs, "drawing_add_section_view", map[string]any{
		"name": "A-A", "parentView": "FRONT", "x1": 80.0, "y1": 100.0, "x2": 160.0, "y2": 100.0,
		"centerXmm": 120.0, "centerYmm": 220.0,
	}, &sec)
	if sec.View.Type != "section" || sec.View.BaseView != "FRONT" || sec.View.Projected {
		t.Fatalf("section view = %+v, want a section off FRONT", sec.View)
	}

	// The cut-away comes back as curves tagged by kind: bold section outline + hatch fill + edges.
	var curves struct {
		Segments []struct {
			Visible bool   `json:"visible"`
			Kind    string `json:"kind"`
		} `json:"segments"`
	}
	callJSON(t, cs, "drawing_view_curves", map[string]any{"view": "A-A"}, &curves)
	var section, hatch int
	for _, s := range curves.Segments {
		switch s.Kind {
		case "section":
			section++
		case "hatch":
			hatch++
		}
	}
	if section == 0 {
		t.Error("section view returned no cut-outline curves")
	}
	if hatch == 0 {
		t.Error("section view returned no hatch fill")
	}
}
