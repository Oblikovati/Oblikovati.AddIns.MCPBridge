// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runLoftCenterline is the live driver for a CENTERLINE loft (Slice 4, kLoftWithCenterline): two
// equal circles lofted along a spine that bows to x=2 — the whole loft bends into a banana along
// the centerline (its cross-sections preserved). Shows the bent spine in the viewport.
func runLoftCenterline(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c0 := c.ids(map[string]any{"sketchIndex": 0, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	if c.err != nil || len(c0) < 2 {
		return fmt.Errorf("bottom circle reply: %v (%v)", c0, c.err)
	}
	c.con("ground", c0[1])
	c.dim("radius", "2 cm", c0[0])
	if err := c.requireConstrainedAt(0); err != nil {
		return err
	}

	var wp struct {
		Index int `json:"index"`
	}
	c.json("create_work_plane", map[string]any{"kind": "plane-offset", "refs": []string{"origin/plane/xy"}, "offset": "40 mm"}, &wp)
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"workPlaneIndex": wp.Index}, &sk)
	c1 := c.ids(map[string]any{"sketchIndex": sk.SketchIndex, "kind": "circle", "points": [][]float64{{0, 0}}, "radius": "2 cm"})
	if c.err != nil || len(c1) < 2 {
		return fmt.Errorf("top circle reply: %v (%v)", c1, c.err)
	}
	c.json("add_sketch_constraint", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "ground", "entities": []uint64{c1[1]}}, nil)
	c.dimAt(sk.SketchIndex, "radius", "2 cm", c1[0])
	if err := c.requireConstrainedAt(sk.SketchIndex); err != nil {
		return err
	}

	// Centerline: a polyline on XZ ((u,v)→(u,0,v)) bowing to x=2 at mid height.
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	c.ids(map[string]any{"sketchIndex": 2, "kind": "polyline", "points": [][]float64{{0, 0}, {2, 2}, {0, 4}}})
	if c.err != nil {
		return c.err
	}

	if err := c.applyFeature("loft", map[string]any{
		"sections":   []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
		"centerline": map[string]any{"pathSketchIndex": 2, "pathIndex": 0},
	}); err != nil {
		return err
	}
	return c.checkCentroidXAtLeast("centerline banana (bent spine)", 0.5)
}

// checkCentroidXAtLeast asserts the active part's centre-of-mass x is at least want cm (a bent
// spine moves it off-axis), printing it for a visual cross-check.
func (c *caller) checkCentroidXAtLeast(tag string, want float64) error {
	var pp struct {
		Centroid [3]float64 `json:"centroid"`
	}
	c.json("get_physical_properties", nil, &pp)
	if c.err != nil {
		return c.err
	}
	fmt.Printf("  %-30s centroid x = %.3f cm  want >= %.1f\n", tag, pp.Centroid[0], want)
	if pp.Centroid[0] < want {
		return fmt.Errorf("%s centroid x %.3f below %.1f (spine not bent)", tag, pp.Centroid[0], want)
	}
	return nil
}
