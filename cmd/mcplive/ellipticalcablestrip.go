// SPDX-License-Identifier: GPL-2.0-only

package main

func runEllipticalCableStrip(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "stripW", "expression": "10 mm"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	pts := semiEllipseFrame(1.5, 2.4, 0.08, 32)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": pts}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "stripW", "operation": "new"}); err != nil {
		return err
	}
	return c.checkVolumeTol("cable-strip", polygonArea(pts), 0.001)
}
