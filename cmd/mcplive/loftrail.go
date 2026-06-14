// SPDX-License-Identifier: GPL-2.0-only

package main

import "fmt"

// runLoftRail is the live driver for a RAIL-guided loft (Slice 3, kLoftWithRails): two equal
// circles lofted with a guide rail that bulges to x=3.5 — the loft follows the rail into a barrel
// on the +X side while the rest stays cylindrical. Shows the rail-driven bulge in the viewport.
func runLoftRail(c *caller) error {
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

	// Rail: a polyline on XZ ((u,v)→(u,0,v)) bulging to x=3.5 at mid height, touching both circles.
	c.json("create_sketch", map[string]any{"plane": "XZ"}, nil)
	c.ids(map[string]any{"sketchIndex": 2, "kind": "polyline", "points": [][]float64{{2, 0}, {3.5, 2}, {2, 4}}})
	if c.err != nil {
		return c.err
	}

	if err := c.applyFeature("loft", map[string]any{
		"sections": []map[string]any{{"sketchIndex": 0, "profileIndex": 0}, {"sketchIndex": sk.SketchIndex, "profileIndex": 0}},
		"rails":    []map[string]any{{"pathSketchIndex": 2, "pathIndex": 0}},
	}); err != nil {
		return err
	}
	// Ruled equal-circle cylinder ≈ 50.3 cm³; the rail bulge clears 53.
	return c.checkVolumeAtLeast("rail-guided bulge (barrel)", 53)
}
