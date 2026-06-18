// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

// TestEndToEndAnalysisMassProperties drives the mass-properties surface over MCP: a 40×30×50 mm box
// reports its volume, mass and inertia (with principal moments) through the live router→model→kernel
// stack (M18-F01, #429).
func TestEndToEndAnalysisMassProperties(t *testing.T) {
	cs := e2eClient(t, seededSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "part", "name": "box.opd"}, nil)
	callJSON(t, cs, "create_sketch", map[string]any{"plane": "XY"}, nil)
	callJSON(t, cs, "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "40 mm", "height": "30 mm"}, nil)
	callJSON(t, cs, "add_feature", map[string]any{
		"kind": "extrude", "args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "50 mm"},
	}, nil)

	var mp struct {
		VolumeMm3            float64    `json:"volumeMm3"`
		MassG                float64    `json:"massG"`
		InertiaXxGmm2        float64    `json:"inertiaXxGmm2"`
		PrincipalMomentsGmm2 [3]float64 `json:"principalMomentsGmm2"`
	}
	callJSON(t, cs, "analysis_mass_properties", map[string]any{"densityGCm3": 2.0}, &mp)

	if math.Abs(mp.VolumeMm3-60000) > 1 {
		t.Errorf("volume = %g mm³, want 60000", mp.VolumeMm3)
	}
	if math.Abs(mp.MassG-120) > 1e-3 {
		t.Errorf("mass = %g g, want 120 (2 g/cm³ × 60 cm³)", mp.MassG)
	}
	if mp.InertiaXxGmm2 <= 0 || mp.PrincipalMomentsGmm2[0] <= 0 || mp.PrincipalMomentsGmm2[0] > mp.PrincipalMomentsGmm2[2] {
		t.Errorf("inertia = %+v, want positive Ixx + ascending principal moments", mp)
	}
}
