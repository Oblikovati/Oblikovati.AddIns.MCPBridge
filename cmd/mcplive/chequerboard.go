// SPDX-License-Identifier: GPL-2.0-only

package main

func runChequerboard(c *caller) error {
	if err := addLiveChequerTiles(c, 0, "new"); err != nil {
		return err
	}
	return c.checkVolumeTol("chequerboard", 32*0.3*0.2*0.08, 0.001)
}
