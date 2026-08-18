// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runSheetMetalProbe builds the mitered corner the live run found self-intersecting and reports
// what the offending faces ARE — their reference-key locations and the body's extent — so a
// failure that the pure-Go fixtures cannot reproduce can be compared against them directly.
func runSheetMetalProbe(c *caller) error {
	if _, err := smSheet(c, "smprobe"); err != nil {
		return err
	}
	edgeX, edgeY, err := smTopEdges(c)
	if err != nil {
		return err
	}
	smReportEdge(c, "first  edge", edgeX)
	smReportEdge(c, "second edge", edgeY)
	if err := c.applyFeature("sheetMetalFlange", map[string]any{
		"edge": edgeX, "height": "10 mm", "radius": "2 mm",
	}); err != nil {
		return err
	}
	if err := c.applyFeature("sheetMetalFlange", map[string]any{
		"edge": edgeY, "height": "10 mm", "radius": "2 mm",
		"applyAutoMiter": true, "miterGap": "0.5 mm",
	}); err != nil {
		return err
	}
	smReportBox(c)
	return smReportProblemFaces(c)
}

// smReportEdge prints where an edge reference key sits, so the driver's edge choice is visible.
func smReportEdge(c *caller, label, key string) {
	for _, e := range smAllEdges(c) {
		if e.Key == key {
			fmt.Printf("  %s at %v\n", label, e.Point)
			return
		}
	}
	fmt.Printf("  %s NOT FOUND\n", label)
}

// smAllEdges lists the active body's edges with their locating points.
func smAllEdges(c *caller) []smEdge {
	var rk struct {
		Bodies []struct{ Edges []smEdge } `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	if len(rk.Bodies) == 0 {
		return nil
	}
	return rk.Bodies[0].Edges
}

// smReportBox prints the body's range box, which pins down where the sheet actually is.
func smReportBox(c *caller) {
	var box struct {
		Min []float64 `json:"min"`
		Max []float64 `json:"max"`
	}
	c.json("body_range_box", map[string]any{"bodyIndex": 0}, &box)
	fmt.Printf("  body box %v .. %v\n", box.Min, box.Max)
}

// smReportProblemFaces joins each reported problem back to the face's locating point.
func smReportProblemFaces(c *caller) error {
	var res struct {
		Valid    bool `json:"valid"`
		Problems []struct {
			Kind  string `json:"kind"`
			Key   string `json:"key"`
			Issue string `json:"issue"`
		} `json:"problems"`
	}
	c.json("body_validate", map[string]any{"bodyIndex": 0, "checkLevel": 2}, &res)
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  valid=%v problems=%d\n", res.Valid, len(res.Problems))
	faces := smAllFaces(c)
	for _, p := range res.Problems {
		fmt.Printf("    %s at %v — %s\n", p.Kind, faces[p.Key], p.Issue)
	}
	return nil
}

// smAllFaces maps every face reference key to its locating point.
func smAllFaces(c *caller) map[string][]float64 {
	var rk struct {
		Bodies []struct {
			Faces []struct {
				Key   string    `json:"key"`
				Point []float64 `json:"point"`
			} `json:"faces"`
		} `json:"bodies"`
	}
	c.json("get_reference_keys", nil, &rk)
	out := map[string][]float64{}
	if len(rk.Bodies) == 0 {
		return out
	}
	for _, f := range rk.Bodies[0].Faces {
		out[f.Key] = f.Point
	}
	return out
}
