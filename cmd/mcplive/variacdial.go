// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runVariacDial(c *caller) error {
	for _, p := range [][2]string{{"dialR", "25 mm"}, {"dialT", "3 mm"}, {"shaftR", "5.5 mm"}, {"screwR", "2.5 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "25 mm", "dialR")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "dialT", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 1, []float64{0, 0}, "5.5 mm", "shaftR")
	for _, p := range regularPolygon(3, 1.6, -math.Pi/2) {
		addConstrainedCircle(c, 1, p, "2.5 mm", "screwR")
	}
	for i := 0; i < 4; i++ {
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": i, "operation": "cut", "extent": "through-all"}); err != nil {
			return err
		}
	}
	want := (math.Pi*2.5*2.5 - math.Pi*0.55*0.55 - 3*math.Pi*0.25*0.25) * 0.3
	return c.checkVolumeTol("variac", want, 0.03)
}
