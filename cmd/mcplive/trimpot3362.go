// SPDX-License-Identifier: GPL-2.0-only

package main

func runTrimpot3362(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-0.3495, -0.33}, {0.3495, 0.33}}, "6.99 mm", "6.6 mm", "4.5 mm", "new"); err != nil {
		return err
	}
	for _, p := range [][]float64{{-0.26, -0.22}, {0.26, -0.22}, {0, 0.22}} {
		if err := addBoxFeature(c, [][]float64{{p[0] - 0.019, p[1] - 0.019}, {p[0] + 0.019, p[1] + 0.019}}, "0.38 mm", "0.38 mm", "0.38 mm", "join"); err != nil {
			return err
		}
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 4, []float64{0, 0}, "2.77 mm", "2.77 mm")
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 4, "profileIndex": 0, "operation": "cut", "extent": "through-all"})
}
