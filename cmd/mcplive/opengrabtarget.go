// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runOpengrabTarget(c *caller) error {
	for _, p := range [][2]string{{"targetSide", "40 mm"}, {"targetT", "1 mm"}, {"cornerHoleR", "1.6 mm"}, {"sideHoleR", "2 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedRect(c, 0, [][]float64{{-2, -2}, {2, 2}}, "targetSide", "targetSide")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "targetT", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	for _, p := range [][2]float64{{-1.69, -1.69}, {-1.69, 1.69}, {1.69, -1.69}, {1.69, 1.69}} {
		addConstrainedCircle(c, 1, []float64{p[0], p[1]}, "1.6 mm", "cornerHoleR")
	}
	for _, p := range [][2]float64{{-1.65, 0}, {1.65, 0}} {
		addConstrainedCircle(c, 1, []float64{p[0], p[1]}, "2 mm", "sideHoleR")
	}
	for i := 0; i < 6; i++ {
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": i, "operation": "cut", "extent": "through-all"}); err != nil {
			return err
		}
	}
	want := 4.0*4.0*0.1 - 4*math.Pi*0.16*0.16*0.1 - 2*math.Pi*0.2*0.2*0.1
	return c.checkVolumeTol("target", want, 0.02)
}
