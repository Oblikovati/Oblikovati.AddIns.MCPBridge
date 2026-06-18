// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	stdmath "math"
	"testing"

	"oblikovati.org/app"
	"oblikovati.org/kernel/ops"
	"oblikovati.org/math"
	"oblikovati.org/model/compdef"
	"oblikovati.org/model/feature"
	"oblikovati.org/model/sketch"
)

// drawingCylinderSession seeds a 2 cm-radius cylinder part "box.opd" so a drawing can dimension
// its circular edge (a box has none) over the MCP bridge.
func drawingCylinderSession(t *testing.T) *app.Session {
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
	sk.Circles().AddByCenterRadius(math.P2(0, 0), 2)
	feature.NewExtrudeFeatures(def.Features()).AddByDistanceExtent(sk, 0, ops.NewBody, func() float64 { return 5 })
	def.Recompute()
	return s
}

// TestEndToEndDrawingRadialDimension drives the radial-dimension surface over MCP: a TOP view of a
// cylinder gets a diameter dimension on its rim, re-measuring the true 40 mm diameter through the
// live router→model→kernel stack (M14-F03 PBI-141, #388).
func TestEndToEndDrawingRadialDimension(t *testing.T) {
	cs := e2eClient(t, drawingCylinderSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "TOP", "orientation": "top", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	var dim struct {
		Dimension struct {
			Type       string  `json:"type"`
			ValueMM    float64 `json:"valueMm"`
			CurveCount int     `json:"curveCount"`
			Text       string  `json:"text"`
		} `json:"dimension"`
	}
	callJSON(t, cs, "drawing_add_radial_dimension", map[string]any{
		"name": "D1", "viewName": "TOP", "type": "diameter", "pickXmm": 120.0, "pickYmm": 100.0,
	}, &dim)

	if dim.Dimension.Type != "diameter" || stdmath.Abs(dim.Dimension.ValueMM-40) > 1e-6 {
		t.Fatalf("radial dimension = %+v, want a diameter Ø40", dim.Dimension)
	}
	if dim.Dimension.CurveCount == 0 || dim.Dimension.Text == "" {
		t.Errorf("radial dimension = %+v, want glyph curves + value text", dim.Dimension)
	}
}
