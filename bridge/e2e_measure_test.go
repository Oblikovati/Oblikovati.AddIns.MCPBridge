// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestEndToEndAnalysisMeasure drives the measurement surface over MCP: an edge of a 40×30×50 mm box
// reports a length in {40,30,50} mm, a face an area in {1200,1500,2000} mm², the minimum distance
// between two faces their gap, the angle between two faces 90°/180°, and a face's loop length its
// perimeter — through the live router→model→kernel stack (M18-F01 PBI-164, #428).
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

	// A box face has exactly one opposite face (gap = a box dimension) and four adjacent (gap 0).
	positives := 0
	for i := 1; i < len(faces); i++ {
		callJSON(t, cs, "analysis_measure", map[string]any{"type": "minDistance", "keyA": faces[0], "keyB": faces[i]}, &m)
		if m.Unit != "mm" || !nearOneOf(m.Value, 0, 30, 40, 50) {
			t.Errorf("minDistance(face0, face%d) = %+v, want one of 0/30/40/50 mm", i, m)
		}
		if m.Value > 0.01 {
			positives++
		}
	}
	if positives != 1 {
		t.Errorf("face0 had %d non-zero face gaps, want 1 (the opposite face)", positives)
	}

	// The angle between two box faces is 90° (adjacent) or 180° (opposite).
	for i := 1; i < len(faces); i++ {
		callJSON(t, cs, "analysis_measure", map[string]any{"type": "angle", "keyA": faces[0], "keyB": faces[i]}, &m)
		if m.Unit != "deg" || (!nearOneOf(m.Value, 90) && !nearOneOf(m.Value, 180)) {
			t.Errorf("angle(face0, face%d) = %+v, want 90 or 180 deg", i, m)
		}
	}

	// A box face's loop length (perimeter) is 2(w+h) ∈ {140,160,180} mm for the 40×30×50 box.
	callJSON(t, cs, "analysis_measure", map[string]any{"type": "loopLength", "keyA": faces[0]}, &m)
	if m.Unit != "mm" || !nearOneOf(m.Value, 140, 160, 180) {
		t.Errorf("loopLength(face0) = %+v, want one of 140/160/180 mm", m)
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
