// SPDX-License-Identifier: GPL-2.0-only

package main

func runIDCTransition(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "pitch", "expression": "2.54 mm"}, nil)
	if err := addBoxFeature(c, [][]float64{{-0.889, 0}, {0.889, 0.74}}, "17.78 mm", "7.4 mm", "6 mm", "new"); err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		x := 0.127 * (float64(i) - 4.5)
		c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
		addConstrainedCircle(c, i+1, []float64{x, 0.37}, "0.64 mm", "pitch/4")
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": i + 1, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
			return err
		}
	}
	if err := addBoxFeature(c, [][]float64{{-0.635, 0.285}, {0.635, 0.37}}, "12.7 mm", "0.8466666667 mm", "8 mm", "cut"); err != nil {
		return err
	}
	for x := 0; x < 5; x++ {
		for y := 0; y < 2; y++ {
			cx := 0.254 * (float64(x) - 2)
			cy := 0.254 * (float64(y) - 0.5)
			if err := addBoxFeature(c, [][]float64{{cx - 0.025, cy - 0.025}, {cx + 0.025, cy + 0.025}}, "0.5 mm", "0.5 mm", "5 mm", "join"); err != nil {
				return err
			}
		}
	}
	return nil
}
