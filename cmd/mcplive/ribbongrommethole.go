// SPDX-License-Identifier: GPL-2.0-only

package main

func runRibbonGrommetHole(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "holeH", "expression": "50 mm"}, nil)
	pts := ribbonGrommetProfile2D(2.72, 0.405, 0.15, 16)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": pts}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "holeH", "operation": "new"}); err != nil {
		return err
	}
	return c.checkVolumeTol("ribbon-hole", polygonArea(pts)*5.0, 0.001)
}
