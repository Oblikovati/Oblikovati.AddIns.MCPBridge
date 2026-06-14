// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runJack(c *caller) error {
	for _, p := range [][2]string{{"jackW", "7 mm"}, {"jackH", "6 mm"}, {"jackL", "6 mm"}, {"boreR", "1.75 mm"}, {"tubeR", "3 mm"}, {"tubeL", "8.5 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedRect(c, 0, [][]float64{{-0.3, -0.35}, {0.3, 0.35}}, "jackH", "jackW")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "jackL", "operation": "new"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 1, []float64{0, 0}, "1.75 mm", "boreR")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 1, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 2, []float64{0, 0}, "3 mm", "tubeR")
	addConstrainedCircle(c, 2, []float64{0, 0}, "1.75 mm", "boreR")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "distance": "tubeL", "operation": "join"}); err != nil {
		return err
	}
	want := (0.6*0.7*0.6 - math.Pi*0.175*0.175*0.6) + math.Pi*(0.3*0.3-0.175*0.175)*0.25
	return c.checkVolumeTol("jack", want, 0.03)
}
