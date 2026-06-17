// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
	"oblikovati.org/math"
	"oblikovati.org/model/compdef"
	"oblikovati.org/model/feature"
	"oblikovati.org/model/sketch"
)

// drawingViewBoxSession seeds a boxed part "box.opd" so a drawing can project base/projected
// views of it over the MCP bridge.
func drawingViewBoxSession(t *testing.T) *app.Session {
	t.Helper()
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("RegisterStandardCommands: %v", err)
	}
	part, err := compdef.AddPart(s.Workspace(), "box.opd", true)
	if err != nil {
		t.Fatalf("AddPart: %v", err)
	}
	def := part.Content().(*compdef.PartComponentDefinition)
	sk := def.Sketches().Add(sketch.XYPlane())
	c0 := sk.Points().Add(math.P2(-2, -3))
	c1 := sk.Points().Add(math.P2(2, -3))
	c2 := sk.Points().Add(math.P2(2, 3))
	c3 := sk.Points().Add(math.P2(-2, 3))
	sk.Lines().Add(c0, c1)
	sk.Lines().Add(c1, c2)
	sk.Lines().Add(c2, c3)
	sk.Lines().Add(c3, c0)
	feature.NewExtrudeFeatures(def.Features()).AddByDistanceExtent(sk, 0, ops.NewBody, func() float64 { return 5 })
	def.Recompute()
	return s
}

// TestEndToEndDrawingViews drives the whole drawing-view surface over MCP: create a drawing,
// reference the boxed part, add a base + projected view, and read the hidden-line curves —
// proving base/projected views and HLR work through the live router→model→kernel stack (#386).
func TestEndToEndDrawingViews(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)

	var base struct {
		View struct {
			Name         string `json:"name"`
			Projected    bool   `json:"projected"`
			VisibleCount int    `json:"visibleCount"`
			HiddenCount  int    `json:"hiddenCount"`
		} `json:"view"`
	}
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, &base)
	if base.View.Name != "FRONT" || base.View.VisibleCount == 0 || base.View.HiddenCount == 0 {
		t.Fatalf("base view = %+v, want FRONT with visible + hidden edges", base.View)
	}

	proj := base
	callJSON(t, cs, "drawing_add_projected_view", map[string]any{
		"name": "RIGHT", "baseView": "FRONT", "direction": "right", "centerXmm": 240.0, "centerYmm": 100.0,
	}, &proj)
	if !proj.View.Projected {
		t.Fatalf("projected view = %+v, want a projected view", proj.View)
	}

	// The curves come back as 2D segments flagged visible/hidden, each keyed to a model edge.
	var curves struct {
		Segments []struct {
			Visible bool   `json:"visible"`
			EdgeKey string `json:"edgeKey"`
		} `json:"segments"`
	}
	callJSON(t, cs, "drawing_view_curves", map[string]any{"view": "FRONT"}, &curves)
	if len(curves.Segments) == 0 {
		t.Fatal("FRONT view returned no drawing curves")
	}
	for _, seg := range curves.Segments {
		if seg.EdgeKey == "" {
			t.Errorf("curve segment %+v carries no source edge key", seg)
		}
	}

	var list struct {
		Views []struct {
			Name string `json:"name"`
		} `json:"views"`
	}
	callJSON(t, cs, "drawing_list_views", nil, &list)
	if len(list.Views) != 2 {
		t.Fatalf("views = %d, want 2 (base + projected)", len(list.Views))
	}
}
