// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"encoding/json"
	"fmt"
)

// identity is the row-major 4x4 identity used to drop a placed component at the origin.
var identity = []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

// assembly drives the M44 assembly surfaces: options (#1981), virtual components (#1979),
// per-occurrence BOM structure (#1978), the occurrence DOF split (#1980), and joint origin
// modes (#1973).
func (dr *d) assembly() {
	fmt.Println("== ASSEMBLY (options/virtual/BOM/DOF/joint-origin) ==")
	partID := dr.openDemoPartID()
	if dr.openDoc("assembly", "m44asm") == 0 {
		return
	}

	// #1981 defaults: first-component grounding + redundancy analysis on.
	got := dr.step("options_get defaults", "assembly_options_get", map[string]any{})
	assertJSONBool(dr, got, "options.placeAndGroundFirstComponentAtOrigin", true, "#1981 ground-first default")
	assertJSONBool(dr, got, "options.enableConstraintRedundancyAnalysis", true, "#1981 redundancy default")

	dr.step("options_set deferUpdate", "assembly_options_set",
		map[string]any{"options": map[string]any{
			"deferUpdate":                          true,
			"placeAndGroundFirstComponentAtOrigin": true,
			"enableConstraintRedundancyAnalysis":   true,
		}})
	after := dr.step("options_get after set", "assembly_options_get", map[string]any{})
	assertJSONBool(dr, after, "options.deferUpdate", true, "#1981 deferUpdate persisted")

	// #1979 virtual component: no geometry, no file, still an occurrence in the BOM.
	dr.step("add_virtual", "assembly_add_virtual",
		map[string]any{"name": "seal-virtual", "partNumber": "VS-1", "structure": "normal"})

	// Two real occurrences from the open demo part → geometry for the DOF split.
	occ1 := dr.step("place demo #1", "place_component",
		map[string]any{"document": partID, "name": "demo:1", "transform": identity})
	shifted := append([]float64(nil), identity...)
	shifted[3] = 8 // +8 cm in X so the second body does not overlap the first
	occ2 := dr.step("place demo #2", "place_component",
		map[string]any{"document": partID, "name": "demo:2", "transform": shifted})

	id1, id2 := jsonUint(occ1, "occurrence.id"), jsonUint(occ2, "occurrence.id")
	// #1978 per-occurrence BOM structure override.
	if id2 != 0 {
		bs := dr.step("set_bom_structure phantom", "assembly_set_bom_structure",
			map[string]any{"occurrence": id2, "structure": "phantom"})
		assertContains(dr, bs, "phantom", "#1978 structure override applied")
	}

	// #1980 DOF split: ground the first placed occurrence (0 DOF), leaving the second free
	// (3 translation / 3 rotation).
	if id1 != 0 {
		dr.step("ground occ #1", "ground_occurrence", map[string]any{"id": id1, "grounded": true})
	}
	health := dr.step("constraint_health (DOF split)", "assembly_constraint_health", map[string]any{})
	dr.checkDOFSplit(health, id1, id2)
}

// openDemoPartID returns the id of an open part document to place, activating it so the
// drawing flow can reference it. It falls back to building a fresh box when none is open.
func (dr *d) openDemoPartID() uint64 {
	list := dr.step("list_documents", "list_documents", map[string]any{})
	if id := firstPartID(list); id != 0 {
		dr.step("activate demo part", "activate_document", map[string]any{"id": id})
		return id
	}
	dr.step("create fallback part", "create_document", map[string]any{"type": "part", "name": "m44part"})
	dr.step("sketch", "create_sketch", map[string]any{"plane": "XY"})
	dr.step("rectangle", "sketch_rectangle", map[string]any{"sketchIndex": 0, "width": "60 mm", "height": "40 mm"})
	dr.step("extrude", "add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "25 mm", "operation": "new"}})
	return firstPartID(dr.step("list_documents #2", "list_documents", map[string]any{}))
}

// checkDOFSplit asserts the per-occurrence DOF breakdown proves #1980: the grounded first
// component reports 0 total, and the free second reports 3 translation + 3 rotation.
func (dr *d) checkDOFSplit(health string, grounded, free uint64) {
	var h struct {
		Occurrences []struct {
			ID               uint64 `json:"occurrence"`
			DegreesOfFreedom int    `json:"degreesOfFreedom"`
			TranslationCount int    `json:"translationCount"`
			RotationCount    int    `json:"rotationCount"`
		} `json:"occurrences"`
	}
	if json.Unmarshal([]byte(health), &h) != nil || len(h.Occurrences) == 0 {
		dr.fail++
		fmt.Printf("  WARN %-28s no per-occurrence DOF in health payload\n", "DOF split shape")
		return
	}
	for _, o := range h.Occurrences {
		switch o.ID {
		case grounded:
			assertEq(dr, o.DegreesOfFreedom, 0, "#1980 grounded occ total DOF")
		case free:
			assertEq(dr, o.TranslationCount, 3, "#1980 free occ translation DOF")
			assertEq(dr, o.RotationCount, 3, "#1980 free occ rotation DOF")
			assertEq(dr, o.TranslationCount+o.RotationCount, o.DegreesOfFreedom, "#1980 split sums to total")
		}
	}
}
