// SPDX-License-Identifier: GPL-2.0-only

package main

func runSmdResistor(c *caller) error {
	for _, p := range [][2]string{{"resZ", "1.2 mm"}, {"bodyX", "5.6 mm"}, {"bodyY", "2.5 mm"}, {"capX", "2.2 mm"}} {
		c.json("add_parameter", map[string]any{"name": p[0], "expression": p[1]}, nil)
	}
	if err := addBoxFeature(c, [][]float64{{-0.28, -0.125}, {0.28, 0.125}}, "bodyX", "bodyY", "resZ", "new"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{-0.50, -0.125}, {-0.28, 0.125}}, "capX", "bodyY", "resZ", "join"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{0.28, -0.125}, {0.50, 0.125}}, "capX", "bodyY", "resZ", "join"); err != nil {
		return err
	}
	return c.checkVolumeTol("smd", 1.0*0.25*0.12, 0.001)
}
