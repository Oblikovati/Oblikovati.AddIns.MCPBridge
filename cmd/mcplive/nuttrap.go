// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runNutTrap(c *caller) error {
	for _, p := range [][2]string{{"screwR", "1.7 mm"}, {"nutR", "3.2 mm"}, {"nutDepth", "2.5 mm"}, {"screwH", "20 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "1.7 mm", "screwR")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "screwH", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 1, "kind": "polyline", "closed": true, "points": regularPolygon(6, 0.32, math.Pi/6)}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "2 * nutDepth", "operation": "join"}); err != nil {
		return err
	}
	vol := func(depthMM float64) float64 {
		return math.Pi*0.17*0.17*2.0 + (3*math.Sqrt(3)*0.32*0.32/2-math.Pi*0.17*0.17)*(2*depthMM/10)
	}
	if err := c.checkVolumeTol("depth=2.5", vol(2.5), 0.03); err != nil {
		return err
	}
	c.json("set_parameter", map[string]any{"name": "nutDepth", "expression": "4 mm"}, nil)
	return c.checkVolumeTol("depth=4", vol(4), 0.03)
}
