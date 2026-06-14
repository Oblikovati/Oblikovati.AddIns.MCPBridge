// SPDX-License-Identifier: GPL-2.0-only

package main

func runSbrRail(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "railLen", "expression": "40 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	pts := sbrRailSection2D()
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": pts}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "railLen - 5 mm", "operation": "new"}); err != nil {
		return err
	}
	return c.checkVolumeTol("sbr16s", polygonArea(pts)*3.5, 0.001)
}
