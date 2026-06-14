// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runHangingHole(c *caller) error {
	for _, p := range [][2]string{{"supportW", "9 mm"}, {"supportH", "5 mm"}, {"holeR", "2.5 mm"}, {"holeH", "14 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedRect(c, 0, [][]float64{{-0.45, -0.45}, {0.45, 0.45}}, "supportW", "supportW")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "supportH", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 1, []float64{0, 0}, "2.5 mm", "holeR")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "distance": "holeH", "operation": "join"}); err != nil {
		return err
	}
	want := 0.9*0.9*0.5 + math.Pi*0.25*0.25*1.4 - math.Pi*0.25*0.25*0.5
	return c.checkVolumeTol("hanging", want, 0.03)
}
