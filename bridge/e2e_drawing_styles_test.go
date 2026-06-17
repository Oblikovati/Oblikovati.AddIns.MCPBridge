// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "testing"

// TestEndToEndDraftingStandardSwitch drives the full stack over MCP: a new drawing defaults
// to ISO (mm, 2 decimals); switching to ANSI returns the ANSI preset (inches, 3 decimals) —
// the PBI-138 acceptance that switching the standard changes the dimension appearance (#385).
func TestEndToEndDraftingStandardSwitch(t *testing.T) {
	cs := e2eClient(t, seededSession(t))

	var created struct {
		Type string `json:"type"`
	}
	callJSON(t, cs, "create_document", map[string]any{"type": "drawing", "name": "styled.odd"}, &created)
	if created.Type != "drawing" {
		t.Fatalf("create_document type = %q, want drawing", created.Type)
	}

	// Default standard is ISO (metric).
	var standards struct {
		Standards []string `json:"standards"`
		Active    string   `json:"active"`
	}
	callJSON(t, cs, "drawing_list_standards", nil, &standards)
	if standards.Active != "iso" || len(standards.Standards) != 2 {
		t.Fatalf("standards = %+v, want active iso + 2 standards", standards)
	}

	var iso struct {
		Style struct {
			Standard  string `json:"standard"`
			Dimension struct {
				Unit          string `json:"unit"`
				DecimalPlaces int    `json:"decimalPlaces"`
			} `json:"dimension"`
		} `json:"style"`
	}
	callJSON(t, cs, "drawing_get_active_style", nil, &iso)
	if iso.Style.Dimension.Unit != "mm" || iso.Style.Dimension.DecimalPlaces != 2 {
		t.Fatalf("ISO dimension = %+v, want mm/2dp", iso.Style.Dimension)
	}

	// Switch to ANSI: the returned preset is inches with 3 decimals.
	ansi := iso
	callJSON(t, cs, "drawing_set_standard", map[string]any{"standard": "ansi"}, &ansi)
	if ansi.Style.Standard != "ansi" || ansi.Style.Dimension.Unit != "in" || ansi.Style.Dimension.DecimalPlaces != 3 {
		t.Fatalf("after switch = %+v, want ansi/in/3dp", ansi.Style.Dimension)
	}
}
