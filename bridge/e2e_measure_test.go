// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestEndToEndAnalysisMeasure drives the measurement surface over MCP: an edge of a 40×30×50 mm box
// reports a length in {40,30,50} mm and a face an area in {1200,1500,2000} mm² through the live
// router→model→kernel stack (M18-F01 PBI-164, #428).
func TestEndToEndAnalysisMeasure(t *testing.T) {
	cs := e2eClient(t, seededSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "box.opd"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "50 mm"},
	}, nil)
	edges, faces := topology(t, cs)

	var m struct {
		Value float64 `json:"value"`
		Unit  string  `json:"unit"`
	}
	callJSON(t, cs, "analysis_measure", map[string]any{"type": "length", "keyA": edges[0]}, &m)
	if m.Unit != "mm" || !nearOneOf(m.Value, 40, 30, 50) {
		t.Errorf("edge length = %+v, want one of 40/30/50 mm", m)
	}
	callJSON(t, cs, "analysis_measure", map[string]any{"type": "area", "keyA": faces[0]}, &m)
	if m.Unit != "mm²" || !nearOneOf(m.Value, 1200, 1500, 2000) {
		t.Errorf("face area = %+v, want one of 1200/1500/2000 mm²", m)
	}
}

func nearOneOf(v float64, candidates ...float64) bool {
	for _, c := range candidates {
		if math.Abs(v-c) < 0.01 {
			return true
		}
	}
	return false
}
