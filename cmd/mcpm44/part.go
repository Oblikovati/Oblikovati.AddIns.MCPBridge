// SPDX-License-Identifier: GPL-2.0-only

package main

import "encoding/json"

// buildParamPart authors a fully-constrained parametric box whose two side lengths are driven
// distance dimensions (boxlen/boxwid). Those DistanceDim constraints are exactly what the
// drawing's model-dimension retrieve (#1991) projects onto a base view, so referencing this
// part gives the retrieve real dimensions to return. It returns the part's full document name.
func (dr *d) buildParamPart() string {
	// Name the part with its .opd full name so the workspace byName key (create name, verbatim)
	// matches the reference the drawing later resolves through workspace.ByName.
	const fullName = "m44param.opd"
	if dr.openDoc("part", fullName) == 0 {
		return ""
	}
	dr.step("param boxlen", "add_parameter", map[string]any{"name": "boxlen", "expression": "70 mm"})
	dr.step("param boxwid", "add_parameter", map[string]any{"name": "boxwid", "expression": "45 mm"})
	sk := jsonInt(dr.step("param sketch", "create_sketch", map[string]any{"plane": "XY"}), "sketchIndex")

	// Rectangle 7cm x 4.5cm (kernel unit = cm); capture each line's endpoint point ids.
	bottom := dr.addLine(sk, 0, 0, 7, 0)  // horizontal → boxlen span
	right := dr.addLine(sk, 7, 0, 7, 4.5) // vertical   → boxwid span
	dr.addLine(sk, 7, 4.5, 0, 4.5)
	dr.addLine(sk, 0, 4.5, 0, 0)

	if len(bottom) == 2 {
		dr.step("dim boxlen", "add_sketch_dimension",
			map[string]any{"sketchIndex": sk, "kind": "distance", "entities": bottom, "expression": "boxlen"})
	}
	if len(right) == 2 {
		dr.step("dim boxwid", "add_sketch_dimension",
			map[string]any{"sketchIndex": sk, "kind": "distance", "entities": right, "expression": "boxwid"})
	}
	dr.step("param extrude", "add_feature", map[string]any{"kind": "extrude",
		"args": map[string]any{"sketchIndex": sk, "profileIndex": 0, "distance": "20 mm", "operation": "new"}})
	return fullName
}

// addLine adds a sketch line and returns its two endpoint point ids.
func (dr *d) addLine(sk int, ax, ay, bx, by float64) []uint64 {
	txt := dr.step("add line", "add_sketch_entity", map[string]any{
		"sketchIndex": sk, "kind": "line", "points": [][]float64{{ax, ay}, {bx, by}}})
	var r struct {
		PointIDs []uint64 `json:"pointIds"`
	}
	_ = json.Unmarshal([]byte(txt), &r)
	return r.PointIDs
}
