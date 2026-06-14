// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runLedBezelRetainer(c *caller) error {
	for _, p := range [][2]string{{"or", "4.5 mm"}, {"ir", "3.2 mm"}, {"h", "4 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "4.5 mm", "or")
	addConstrainedCircle(c, 0, []float64{0, 0}, "3.2 mm", "ir")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "h", "operation": "new"}); err != nil {
		return err
	}
	return c.checkVolumeTol("led-retainer", math.Pi*(0.45*0.45-0.32*0.32)*0.4, 0.03)
}
