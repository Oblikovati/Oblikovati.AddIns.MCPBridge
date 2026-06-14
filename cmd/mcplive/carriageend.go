// SPDX-License-Identifier: GPL-2.0-only

package main

func runCarriageEnd(c *caller) error {
	for i, x := range []float64{-0.9, 0.9} {
		op := "join"
		if i == 0 {
			op = "new"
		}
		if err := addBoxFeature(c, [][]float64{{x - 0.4, -0.05}, {x + 0.4, 0.65}}, "8 mm", "7 mm", "5 mm", op); err != nil {
			return err
		}
		if err := addBoxFeature(c, [][]float64{{x - 0.225, 0}, {x + 0.225, 0.35}}, "4.5 mm", "3.5 mm", "7 mm", "cut"); err != nil {
			return err
		}
	}
	return nil
}
