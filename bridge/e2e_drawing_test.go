// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/model/attr"
	"oblikovati.org/model/compdef"
)

// TestEndToEndDrawingSheetsAndTitleBlock drives the full stack over MCP: create a drawing,
// list/add sheets, point it at the seeded part, and read the title block — proving the
// drawing.* tools resolve the referenced model's iProperties end to end (M14-F01, #384).
func TestEndToEndDrawingSheetsAndTitleBlock(t *testing.T) {
	s := seededSession(t)
	// Stamp an iProperty on the seeded part ("e2e.obk") so the title block has something
	// to resolve once the drawing references it.
	part := s.ActiveDocument().Content().(*compdef.PartComponentDefinition)
	part.Properties().Set(attr.DesignTracking).Put("Part Number", attr.StringValue("PN-42"))

	cs := e2eClient(t, s)

	// Creating a drawing makes it the active document.
	var created struct {
		Type string `json:"type"`
	}
	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "sheet.odd"}, &created)
	if created.Type != "drawing" {
		t.Fatalf("create_document type = %q, want drawing", created.Type)
	}

	// The new drawing has one default A3-landscape sheet.
	var sheets struct {
		Sheets []struct {
			Name        string `json:"name"`
			Size        string `json:"size"`
			Orientation string `json:"orientation"`
			Active      bool   `json:"active"`
		} `json:"sheets"`
	}
	callJSON(t, cs, "drawing_list_sheets", nil, &sheets)
	if len(sheets.Sheets) != 1 || sheets.Sheets[0].Size != "a3" || !sheets.Sheets[0].Active {
		t.Fatalf("default sheets = %+v, want one active A3", sheets.Sheets)
	}

	// Add an A4 portrait sheet.
	var added struct {
		Sheet struct {
			Name   string  `json:"name"`
			Size   string  `json:"size"`
			Height float64 `json:"heightMm"`
		} `json:"sheet"`
	}
	callJSON(t, cs, "drawing_add_sheet", map[string]any{"size": "a4", "orientation": "portrait"}, &added)
	if added.Sheet.Name != "Sheet:2" || added.Sheet.Size != "a4" || added.Sheet.Height != 297 {
		t.Fatalf("added sheet = %+v, want A4 Sheet:2 297 tall", added.Sheet)
	}

	// Point the drawing at the seeded part.
	var ref struct {
		ModelReference string `json:"modelReference"`
	}
	callJSON(t, cs, "drawing_set_model_reference", map[string]any{"fullDocumentName": "e2e.obk"}, &ref)
	if ref.ModelReference != "e2e.obk" {
		t.Fatalf("model reference = %q, want e2e.obk", ref.ModelReference)
	}

	// The title block resolves the part's iProperties.
	var fields struct {
		Fields []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"fields"`
	}
	callJSON(t, cs, "drawing_title_block_fields", nil, &fields)
	got := map[string]string{}
	for _, f := range fields.Fields {
		got[f.Name] = f.Value
	}
	if got["Part Number"] != "PN-42" {
		t.Fatalf("title block Part Number = %q, want PN-42 (fields=%+v)", got["Part Number"], fields.Fields)
	}
}
