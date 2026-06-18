// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingSketches drives the drawing-sketch surface over MCP: add a sketch, add a
// rectangle + a circle, and confirm the entity/curve counts through the live router→model stack
// (M14-F08, #638).
func TestEndToEndDrawingSketches(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_add_sketch", map[string]any{"name": "S1"}, nil)

	var sk struct {
		Sketch struct {
			Name        string `json:"name"`
			EntityCount int    `json:"entityCount"`
			CurveCount  int    `json:"curveCount"`
		} `json:"sketch"`
	}
	callJSON(t, cs, "drawing_add_sketch_entity", map[string]any{
		"sketchName": "S1", "kind": "rectangle", "points": [][2]float64{{10, 20}, {60, 50}},
	}, &sk)
	callJSON(t, cs, "drawing_add_sketch_entity", map[string]any{
		"sketchName": "S1", "kind": "circle", "points": [][2]float64{{100, 100}}, "radiusMm": 12.0,
	}, &sk)

	if sk.Sketch.Name != "S1" || sk.Sketch.EntityCount != 2 || sk.Sketch.CurveCount < 4+8 {
		t.Fatalf("sketch = %+v, want S1 with 2 entities + rectangle/circle curves", sk.Sketch)
	}

	// A cross-hatch region adds many fill-line curves to the same sketch.
	before := sk.Sketch.CurveCount
	callJSON(t, cs, "drawing_add_hatch_region", map[string]any{
		"sketchName": "S1", "xmm": 10.0, "ymm": 20.0, "widthMm": 50.0, "heightMm": 30.0, "pattern": "cross",
	}, &sk)
	if sk.Sketch.CurveCount <= before {
		t.Fatalf("after a hatch region the sketch has %d curves, want more than before (%d)", sk.Sketch.CurveCount, before)
	}
}
