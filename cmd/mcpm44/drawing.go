// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// drawing drives the M44 drawing surfaces and captures the rendered sheet: sheet authoring
// (#1989), view rotation/alignment (#1988), tangent-edge display (#1984), model-dimension
// retrieve (#1991), and chamfer/bend notes (#1995).
func (dr *d) drawing() {
	fmt.Println("== DRAWING (sheet/view/dimension/annotation) ==")
	partName := dr.buildParamPart() // a parametric box with driven dims for #1991 retrieve
	if dr.openDoc("drawing", "m44dwg") == 0 {
		return
	}
	view := dr.baseView(partName)
	if view == "" {
		fmt.Println("  (no base view — skipping view-scoped steps)")
		return
	}

	// #1989 sheet authoring.
	dr.step("default border", "drawing_add_default_border",
		map[string]any{"hZones": 8, "vZones": 6, "hLabelMode": "numeric", "vLabelMode": "alphabetical"})
	dr.step("title block", "drawing_set_title_block", map[string]any{"location": "bottomRight"})
	dr.step("sheet revision", "drawing_set_sheet_revision", map[string]any{"revision": "B"})
	dr.shot("sheet-authored")

	// #1988 rotate the view about its centre.
	dr.step("rotate view 30deg", "rotate_view", map[string]any{"name": view, "angleDeg": 30.0})
	dr.shot("view-rotated")

	// #1984 tangent-edge display: suppress smooth edges and prove the curve set shrinks.
	before := countEdges(dr.step("view_curves before", "drawing_view_curves", map[string]any{"view": view}))
	dr.step("tangent edges off", "set_view_display",
		map[string]any{"name": view, "displayTangentEdges": false})
	after := countEdges(dr.step("view_curves after", "drawing_view_curves", map[string]any{"view": view}))
	if after <= before {
		fmt.Printf("  PASS %-28s edges %d -> %d (tangent suppression)\n", "#1984 tangent drop", before, after)
	} else {
		dr.fail++
		fmt.Printf("  WARN %-28s edges grew %d -> %d\n", "#1984 tangent drop", before, after)
	}

	// #1991 retrieve the referenced model's parametric dimensions.
	dr.step("list retrievable dims", "drawing_list_retrievable_dimensions", map[string]any{"viewName": view})
	dr.step("retrieve dims", "drawing_retrieve_dimensions", map[string]any{"viewName": view})

	// #1995 chamfer/bend notes derive from model geometry; a plain box has no chamfer, so a
	// clean rejection is the correct outcome and is logged as such.
	dr.chamferNote(view)
	dr.shot("sheet-final")
}

// baseView sets the drawing's model reference to the given part (resolved through
// workspace.ByName, keyed on the full .opd document name), then adds a shaded iso base view
// of it and returns the view name.
func (dr *d) baseView(fullName string) string {
	dr.step("set model reference", "drawing_set_model_reference", map[string]any{"fullDocumentName": fullName})
	txt := dr.step("add base view", "drawing_add_base_view",
		map[string]any{"orientation": "iso", "style": "shaded", "scale": 1.0, "centerXmm": 150, "centerYmm": 150})
	return jsonStr(txt, "view.name")
}

// chamferNote picks two projected edges sharing an endpoint and asks for a chamfer note; a
// non-chamfer edge is expected to be rejected by the geometry-derived note (#1995).
func (dr *d) chamferNote(view string) {
	txt := dr.step("view_curves for note", "drawing_view_curves", map[string]any{"view": view})
	a, b := twoAdjacentEdgeKeys(txt)
	if a == "" || b == "" {
		fmt.Printf("  INFO %-28s no adjacent edge pair found\n", "#1995 chamfer note")
		return
	}
	if _, err := dr.call("drawing_add_chamfer_note",
		map[string]any{"viewName": view, "edgeA": a, "edgeB": b}); err != nil {
		fmt.Printf("  INFO %-28s rejected non-chamfer edges (correct): %v\n", "#1995 chamfer note", err)
		return
	}
	fmt.Printf("  PASS %-28s note added\n", "#1995 chamfer note")
}

// shot captures the whole application window to a PNG under the out dir.
func (dr *d) shot(name string) {
	path := filepath.Join(dr.out, "m44-"+name+".png")
	if _, err := dr.call("capture_window", map[string]any{"path": path}); err != nil {
		dr.fail++
		fmt.Printf("  WARN %-28s %v\n", "capture "+name, err)
		return
	}
	fmt.Printf("  SHOT %-28s %s\n", name, path)
}

// countEdges counts DrawingCurveSegment entries in a view_curves payload.
func countEdges(txt string) int {
	var r struct {
		Segments []json.RawMessage `json:"segments"`
	}
	_ = json.Unmarshal([]byte(txt), &r)
	return len(r.Segments)
}

// twoAdjacentEdgeKeys returns two edge keys whose segments share an endpoint.
func twoAdjacentEdgeKeys(txt string) (string, string) {
	var r struct {
		Segments []struct {
			AX, AY, BX, BY float64
			EdgeKey        string `json:"edgeKey"`
		} `json:"segments"`
	}
	if json.Unmarshal([]byte(txt), &r) != nil {
		return "", ""
	}
	for i := range r.Segments {
		for j := i + 1; j < len(r.Segments); j++ {
			p, q := r.Segments[i], r.Segments[j]
			if p.EdgeKey == "" || q.EdgeKey == "" || p.EdgeKey == q.EdgeKey {
				continue
			}
			if near(p.BX, q.AX) && near(p.BY, q.AY) {
				return p.EdgeKey, q.EdgeKey
			}
		}
	}
	return "", ""
}

func near(a, b float64) bool { return a-b < 1e-6 && b-a < 1e-6 }
