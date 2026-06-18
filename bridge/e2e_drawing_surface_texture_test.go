// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDrawingSurfaceTexture drives the surface-texture surface over MCP: a machined surface
// texture symbol with a roughness value is placed on a sheet, producing a checkmark annotation
// through the live router→model→kernel stack (M14-F03 PBI-142, #389).
func TestEndToEndDrawingSurfaceTexture(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)

	var st struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_surface_texture", map[string]any{
		"name": "ST", "xmm": 80.0, "ymm": 80.0, "roughness": "1.6", "materialRemoval": "required",
	}, &st)

	if st.Annotation.Kind != "surfaceTexture" {
		t.Errorf("annotation kind = %q, want surfaceTexture", st.Annotation.Kind)
	}
	if st.Annotation.CurveCount < 3 {
		t.Errorf("surface texture = %d curves, want a checkmark glyph", st.Annotation.CurveCount)
	}
}
