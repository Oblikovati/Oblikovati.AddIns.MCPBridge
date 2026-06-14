// SPDX-License-Identifier: GPL-2.0-only

package main

func runDimension(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-0.75, -0.015}, {0.75, 0.015}}, "15 mm", "0.3 mm", "0.3 mm", "new"); err != nil {
		return err
	}
	if err := addLivePolylinePrism(c, [][]float64{{-0.7, -0.08}, {-0.7, 0.08}, {-0.95, 0}}, "0.3 mm", "join"); err != nil {
		return err
	}
	return addLivePolylinePrism(c, [][]float64{{0.7, -0.08}, {0.7, 0.08}, {0.95, 0}}, "0.3 mm", "join")
}
