// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runPolyhole(c *caller) error {
	for _, p := range [][2]string{{"polyR", "2.5 mm"}, {"polyH", "12 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	pts := regularPolygon(8, 0.25, math.Pi/8)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": pts}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "polyH", "operation": "new"}); err != nil {
		return err
	}
	return c.checkVolumeTol("polyhole", polygonArea(pts)*1.2, 0.001)
}
