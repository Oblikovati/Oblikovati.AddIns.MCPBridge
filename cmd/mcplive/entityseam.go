// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"sort"
	"time"
)

// runEntitySeam is the live check for Oblikovati#1624 (audit I1): sketch entities
// now serialize through a paired codec registry and name themselves over the API
// via the Kind capability — no consumer type switch, no default: save-time
// failure. It creates every wire-creatable 2D and 3D entity kind, enumerates
// them, round-trips the document through a real .obk save → force-close →
// reopen, and requires the enumerated kind census to survive identically. The
// drift class that shipped #1416 (an encode half registered without its decode
// half) fails here at the live seam instead of corrupting a user's save.
func runEntitySeam(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addSeam2DEntities(c)
	addSeam3DEntities(c)
	before2D, before3D := entityKindCensus(c, "list_sketch_entities"), entityKindCensus(c, "list_sketch3d_entities")
	if c.err != nil {
		return c.err
	}
	if err := roundTripSeamDocument(c); err != nil {
		return err
	}
	after2D, after3D := entityKindCensus(c, "list_sketch_entities"), entityKindCensus(c, "list_sketch3d_entities")
	if c.err != nil {
		return c.err
	}
	if err := requireSameCensus("2D", before2D, after2D); err != nil {
		return err
	}
	if err := requireSameCensus("3D", before3D, after3D); err != nil {
		return err
	}
	c.json("execute_command", map[string]any{"id": "View.Home"}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-entityseam.png"}, nil)
	return c.err
}

// addSeam2DEntities places one of every wire-creatable 2D kind, spaced apart so
// none of them interact: the primitive curves, the whole spline family (the
// offset spline references the fit spline as its parent), the corner blends
// (which trim their two host lines), an equation curve, and a text entity.
func addSeam2DEntities(c *caller) {
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0}, {2, 0}}}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{5, 5}}}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{4, 0}}, "radius": "5 mm"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "arc", "points": [][]float64{{7, 0}, {7.5, 0}, {7, 0.5}}, "ccw": true}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "ellipse", "points": [][]float64{{10, 0}}, "axis": []float64{1, 0}, "majorRadius": "10 mm", "minorRadius": "5 mm"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "ellipticalArc", "points": [][]float64{{13, 0}}, "axis": []float64{1, 0}, "majorRadius": "10 mm", "minorRadius": "5 mm", "startAngle": "0 deg", "endAngle": "90 deg"}, nil)
	spline := c.ids(map[string]any{"sketchIndex": 0, "kind": "spline", "points": [][]float64{{0, 3}, {1, 3.6}, {2, 3}}})
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "controlPointSpline", "points": [][]float64{{4, 3}, {5, 4}, {6, 3}, {7, 4}}}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "fixedSpline", "points": [][]float64{{9, 3}, {10, 3.6}, {11, 3}}}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "equationCurve", "xExpr": "t", "yExpr": "0.2 * t * t", "t0": 0, "t1": 2}, nil)
	if len(spline) > 0 {
		c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "offsetSpline", "entityRefs": []uint64{spline[0]}, "radius": "2 mm"}, nil)
	}
	addSeamCornerBlends(c)
	c.json("add_sketch_text", map[string]any{"sketchIndex": 0, "anchor": []float64{6, 6}, "text": "I1", "height": "5 mm"}, nil)
}

// addSeamCornerBlends builds two L-corners and blends one with a fillet and the
// other with a chamfer — the two kinds that consume existing entity refs.
func addSeamCornerBlends(c *caller) {
	f1 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 6}, {1, 6}}})
	f2 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{1, 6}, {1, 7}}})
	if len(f1) > 0 && len(f2) > 0 {
		c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "fillet", "entityRefs": []uint64{f1[0], f2[0]}, "radius": "2 mm"}, nil)
	}
	h1 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{3, 6}, {4, 6}}})
	h2 := c.ids(map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{4, 6}, {4, 7}}})
	if len(h1) > 0 && len(h2) > 0 {
		c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "chamfer", "entityRefs": []uint64{h1[0], h2[0]}, "radius": "1 mm"}, nil)
	}
}

// addSeam3DEntities creates a 3D sketch and one of every wire-creatable 3D kind:
// primitives, conics, the spline family, an equation curve, a helix, and a bend
// (which consumes two connected line refs).
func addSeam3DEntities(c *caller) {
	c.json("create_sketch3d", map[string]any{"name": "seam3d"}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "point", "points": [][]float64{{0, 0, 2}}}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 0, 0}, {1, 1, 1}}}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{3, 0, 0}}, "axis": []float64{0, 0, 1}, "radius": "5 mm"}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "arc", "points": [][]float64{{6, 0, 0}, {6.5, 0, 0}, {6, 0.5, 0}}}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "ellipse", "points": [][]float64{{9, 0, 0}}, "majorRadius": "10 mm", "minorRadius": "5 mm"}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "ellipticalArc", "points": [][]float64{{12, 0, 0}}, "majorRadius": "10 mm", "minorRadius": "5 mm", "startAngle": "0 deg", "sweepAngle": "90 deg"}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "spline", "points": [][]float64{{0, 3, 0}, {1, 3.6, 0.5}, {2, 3, 1}}}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "controlPointSpline", "points": [][]float64{{4, 3, 0}, {5, 4, 0.5}, {6, 3, 1}, {7, 4, 1.5}}}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "fixedSpline", "points": [][]float64{{9, 3, 0}, {10, 3.6, 0.5}, {11, 3, 1}}}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "equationCurve", "xExpr": "t", "yExpr": "0.5", "zExpr": "0.3 * t", "t0": 0, "t1": 2}, nil)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "helical", "points": [][]float64{{14, 0, 0}}, "radius": "4 mm", "pitch": "5 mm", "revolutions": 3, "mode": "pitchRevolution"}, nil)
	addSeam3DBend(c)
}

// addSeam3DBend blends the corner of two connected 3D lines with a tangent arc.
func addSeam3DBend(c *caller) {
	var b1, b2 struct {
		EntityID uint64 `json:"entityId"`
	}
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{0, 6, 0}, {1, 6, 0}}}, &b1)
	c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "line", "points": [][]float64{{1, 6, 0}, {1, 7, 0}}}, &b2)
	if b1.EntityID != 0 && b2.EntityID != 0 {
		c.json("add_sketch3d_entity", map[string]any{"sketchIndex": 0, "kind": "bend", "lines": []uint64{b1.EntityID, b2.EntityID}, "radius": "2 mm"}, nil)
	}
}

// entityKindCensus enumerates sketch 0 through the given list tool and counts
// the entities by their self-reported kind — the capability the router now
// derives every enumeration row from.
func entityKindCensus(c *caller, tool string) map[string]int {
	var res struct {
		Entities []struct {
			Kind string `json:"kind"`
		} `json:"entities"`
	}
	c.json(tool, map[string]any{"sketchIndex": 0}, &res)
	census := map[string]int{}
	for _, e := range res.Entities {
		census[e.Kind]++
	}
	return census
}

// roundTripSeamDocument saves the document to a fresh .obk, force-closes
// everything, and reopens the file — the full codec-registry encode/decode path.
func roundTripSeamDocument(c *caller) error {
	path := fmt.Sprintf("/tmp/oblikovati-entityseam-%d.obk", time.Now().UnixNano()%1000000)
	c.json("documents_save_as", map[string]any{"document": c.doc.ID, "newFullDocumentName": path}, nil)
	c.json("close_all_documents", map[string]any{"force": true}, nil)
	var opened struct {
		ID uint64 `json:"id"`
	}
	c.json("documents_open", map[string]any{"fullDocumentName": path, "visible": true}, &opened)
	if c.err != nil {
		return c.err
	}
	c.json("activate_document", map[string]any{"id": opened.ID}, nil)
	fmt.Printf("  round-tripped %s (reopened as document %d)\n", path, opened.ID)
	return c.err
}

// requireSameCensus asserts the post-reload kind census matches pre-save exactly:
// a kind lost (decode half missing) or duplicated on reload fails with the kind named.
func requireSameCensus(dim string, before, after map[string]int) error {
	kinds := make([]string, 0, len(before)+len(after))
	for k := range before {
		kinds = append(kinds, k)
	}
	for k := range after {
		if _, dup := before[k]; !dup {
			kinds = append(kinds, k)
		}
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		if before[k] != after[k] {
			return fmt.Errorf("%s kind %q: %d before save, %d after reload — codec round-trip drift (#1624)", dim, k, before[k], after[k])
		}
	}
	fmt.Printf("  %s census stable across save/reload: %v\n", dim, censusLine(before, kinds))
	return nil
}

// censusLine renders a census in stable kind order for the log.
func censusLine(census map[string]int, kinds []string) string {
	out := ""
	for _, k := range kinds {
		if census[k] == 0 {
			continue
		}
		out += fmt.Sprintf("%s×%d ", k, census[k])
	}
	return out
}
