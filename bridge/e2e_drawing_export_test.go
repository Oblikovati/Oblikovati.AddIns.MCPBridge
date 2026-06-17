// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEndToEndDrawingExportDXF drives the full stack over MCP: build a boxed part, create a
// drawing with a base view, and export the sheet to DXF — proving the drawing → DXF path (view
// edges on Visible/Hidden layers + the title block) through the live router→model→codec stack
// (M14-F05, #392).
func TestEndToEndDrawingExportDXF(t *testing.T) {
	cs := e2eClient(t, drawingViewBoxSession(t))

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "box.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "box.opd"}, nil)
	callJSON(t, cs, "drawing_add_base_view", map[string]any{
		"name": "FRONT", "orientation": "front", "scale": 2.0, "centerXmm": 120.0, "centerYmm": 100.0,
	}, nil)

	path := filepath.ToSlash(t.TempDir()) + "/box.dxf"
	var res struct {
		Path     string `json:"path"`
		Entities int    `json:"entities"`
	}
	callJSON(t, cs, "drawing_export_dxf", map[string]any{"path": path, "version": "r2018"}, &res)
	if res.Entities == 0 {
		t.Fatalf("export wrote no entities: %+v", res)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported DXF: %v", err)
	}
	dxf := string(data)
	for _, want := range []string{"SECTION", "LINE", "Visible", "Hidden", "TitleBlock"} {
		if !strings.Contains(dxf, want) {
			t.Errorf("exported DXF missing %q", want)
		}
	}
}
