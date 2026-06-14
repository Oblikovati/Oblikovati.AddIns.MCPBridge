// SPDX-License-Identifier: GPL-2.0-only

package main

func runPiCutout(c *caller) error {
	for _, p := range [][2]string{{"baseW", "9 mm"}, {"stemW", "1.2 mm"}, {"baseH", "3.5 mm"}, {"gap", "8.6 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	pts := stadiumBandPoints2D(0.35, 0, 0.35, 0.18, 24)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": pts}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "baseH", "operation": "new"}); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-0.45, -0.55}, {0.45, -0.43}}, "baseW", "stemW", "9 mm", "join"); err != nil {
		return err
	}
	return addBoxFeature(c, [][]float64{{-0.45, 0.43}, {0.45, 0.55}}, "baseW", "stemW", "9 mm", "join")
}
