// SPDX-License-Identifier: GPL-2.0-only

package main

func runRadialProfile(c *caller) error {
	return addLivePolylinePrism(c, [][]float64{{0.16, 0}, {0.5, 0}, {0.5, 1.0}, {0.42, 1.16}, {0.22, 1.16}, {0.16, 1.0}}, "0.6 mm", "new")
}
