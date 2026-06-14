// SPDX-License-Identifier: GPL-2.0-only

package main

func runFlatFlex(c *caller) error {
	for _, p := range [][2]string{{"slotW", "11.8 mm"}, {"latchW", "17 mm"}, {"latchT", "1.4 mm"}, {"backW", "16 mm"}, {"midW", "12 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	if err := addBoxFeature(c, [][]float64{{-0.85, -0.27}, {0.85, -0.13}}, "latchW", "latchT", "1.2 mm", "new"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-0.59, -0.32}, {0.59, -0.08}}, "slotW", "2.4 mm", "2 mm", "cut"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-0.8, -0.27}, {0.8, 0.13}}, "backW", "4 mm", "2.5 mm", "join"); err != nil {
		return err
	}
	return addBoxFeature(c, [][]float64{{-0.6, 0.13}, {0.6, 0.29}}, "midW", "1.6 mm", "1.2 mm", "join")
}
