// SPDX-License-Identifier: GPL-2.0-only

package main

func runRibbonGrommet(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "grommetT", "expression": "3 mm"}, nil)
	outer := ribbonGrommetProfile2D(2.75, 0.42, 0.16, 16)
	inner := [][]float64{{-0.95, 0.08}, {0.95, 0.08}, {0.95, 0.24}, {-0.95, 0.24}}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": outer}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "grommetT", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "polyline", "closed": true, "points": inner}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
		return err
	}
	return c.checkVolumeTol("ribbon", (polygonArea(outer)-polygonArea(inner))*0.3, 0.02)
}
