// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runDoorLatchStl(c *caller) error {
	for _, p := range [][2]string{{"length", "35 mm"}, {"width", "12 mm"}, {"th", "5 mm"}, {"bossH", "14.25 mm"}, {"screwR", "2.2 mm"}, {"nutR", "4.2 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	if err := addBoxFeature(c, [][]float64{{-1.75, -0.6}, {1.75, 0.6}}, "length", "width", "th", "new"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-1.75, -0.2}, {1.75, 0.2}}, "length", "4 mm", "8.5 mm", "join"); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 2, []float64{0, 0}, "6 mm", "width/2")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "distance": "bossH", "operation": "join"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 3, []float64{0, 0}, "2.2 mm", "screwR")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 3, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 4, "kind": "polyline", "closed": true, "points": regularPolygon(6, 0.42, math.Pi/6)}, nil)
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 4, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
}
