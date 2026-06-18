// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/app"
)

// TestEndToEndDrawingPartsList drives the parts-list surface over MCP: an assembly of two distinct
// placed parts is documented by a drawing whose parts list reflects the parts-only BOM (two rows)
// through the live router→model→kernel stack (M14-F04 PBI-143, #390).
func TestEndToEndDrawingPartsList(t *testing.T) {
	s := app.NewSession()
	if err := app.RegisterStandardCommands(s); err != nil {
		t.Fatalf("commands: %v", err)
	}
	cs := e2eClient(t, s)

	p1 := newPartDoc(t, cs, "p1.obk")
	p2 := newPartDoc(t, cs, "p2.obk")

	var asm struct {
		ID uint64 `json:"id"`
	}
	callJSON(t, cs, "create_document", map[string]any{"type": "assembly", "name": "asm.obk"}, &asm)
	callJSON(t, cs, "activate_document", map[string]any{"id": asm.ID}, nil)
	placeComponent(t, cs, "place_component", map[string]any{"document": p1, "name": "c1", "transform": identityCells})
	placeComponent(t, cs, "place_component", map[string]any{"document": p2, "name": "c2", "transform": identityCells})

	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "asm.odd"}, nil)
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "asm.obk"}, nil)

	var pl struct {
		Annotation struct {
			Kind       string `json:"kind"`
			CurveCount int    `json:"curveCount"`
			RowCount   int    `json:"rowCount"`
		} `json:"annotation"`
	}
	callJSON(t, cs, "drawing_add_parts_list", map[string]any{"name": "PL", "xmm": 40.0, "ymm": 260.0}, &pl)

	if pl.Annotation.Kind != "partsList" || pl.Annotation.CurveCount == 0 {
		t.Fatalf("parts list = %+v, want a partsList with grid geometry", pl.Annotation)
	}
	if pl.Annotation.RowCount != 2 {
		t.Errorf("parts list rowCount = %d, want 2 (two distinct parts)", pl.Annotation.RowCount)
	}
}
