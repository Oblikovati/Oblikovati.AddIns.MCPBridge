// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

// TestEndToEndDrawingAnnotations drives the CoG-marker and revision-cloud annotations over MCP: a
// drawing of a boxed part gets a base view, a centre-of-gravity marker (from the model's mass
// properties) and a revision cloud, then lists them — exercising the annotation subsystem through
// the live router→model→kernel stack (M14-F02 #813).
func TestEndToEndDrawingAnnotations(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	type annResp struct {
		Annotation struct {
			Kind       string `json:"kind"`
			ViewName   string `json:"viewName"`
			Tag        string `json:"tag"`
			CurveCount int    `json:"curveCount"`
		} `json:"annotation"`
	}
	var cog, cloud annResp
	callJSON(t, cs, "drawing_add_cog_marker", map[string]any{"name": "CG", "viewName": "FRONT"}, &cog)
	if cog.Annotation.Kind != "cog" || cog.Annotation.ViewName != "FRONT" || cog.Annotation.CurveCount == 0 {
		t.Fatalf("CoG marker = %+v, want a cog on FRONT with glyph curves", cog.Annotation)
	}
	callJSON(t, cs, "drawing_add_revision_cloud", map[string]any{
		"name": "REV", "xmm": 40.0, "ymm": 40.0, "widthMm": 60.0, "heightMm": 40.0, "tag": "A",
	}, &cloud)
	if cloud.Annotation.Kind != "revisionCloud" || cloud.Annotation.Tag != "A" || cloud.Annotation.CurveCount == 0 {
		t.Fatalf("revision cloud = %+v, want a tagged scalloped cloud", cloud.Annotation)
	}

	var list struct {
		Annotations []struct {
			Name string `json:"name"`
		} `json:"annotations"`
	}
	callJSON(t, cs, "drawing_list_annotations", nil, &list)
	if len(list.Annotations) != 2 {
		t.Fatalf("annotations = %d, want 2", len(list.Annotations))
	}
}
