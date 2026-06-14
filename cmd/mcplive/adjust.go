// SPDX-License-Identifier: GPL-2.0-only

package main

func runAdjust(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "2.77 mm", "2.77 mm")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "1.78 mm", "operation": "new"}); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-0.16, -0.032}, {0.16, 0.032}}, "3.2 mm", "0.64 mm", "1.1 mm", "cut"); err != nil {
		return err
	}
	return addBoxFeature(c, [][]float64{{-0.032, -0.16}, {0.032, 0.16}}, "0.64 mm", "3.2 mm", "1.1 mm", "cut")
}
