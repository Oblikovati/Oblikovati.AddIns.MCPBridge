// SPDX-License-Identifier: GPL-2.0-only

package main

func runSmdTant(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-0.36, -0.21}, {0.36, 0.21}}, "7.2 mm", "4.2 mm", "2.4 mm", "new"); err != nil {
		return err
	}
	for _, x := range []float64{-0.41, 0.41} {
		if err := addBoxFeature(c, [][]float64{{x - 0.12, -0.17}, {x + 0.12, 0.17}}, "2.4 mm", "3.4 mm", "0.5 mm", "join"); err != nil {
			return err
		}
	}
	if err := addBoxFeature(c, [][]float64{{-0.17, -0.21}, {0.17, 0.21}}, "3.4 mm", "4.2 mm", "1 mm", "cut"); err != nil {
		return err
	}
	return addBoxFeature(c, [][]float64{{-0.31, -0.15}, {-0.23, 0.15}}, "0.8 mm", "3 mm", "0.1 mm", "join")
}
