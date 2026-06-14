// SPDX-License-Identifier: GPL-2.0-only

package main

func runSmdDiode(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-0.23, -0.14}, {0.23, 0.14}}, "4.6 mm", "2.8 mm", "1.6 mm", "new"); err != nil {
		return err
	}
	for _, x := range []float64{-0.26, 0.26} {
		if err := addBoxFeature(c, [][]float64{{x - 0.09, -0.12}, {x + 0.09, 0.12}}, "1.8 mm", "2.4 mm", "0.4 mm", "join"); err != nil {
			return err
		}
	}
	return addBoxFeature(c, [][]float64{{-0.11, -0.14}, {0.11, 0.14}}, "2.2 mm", "2.8 mm", "0.8 mm", "cut")
}
