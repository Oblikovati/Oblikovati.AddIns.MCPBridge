// SPDX-License-Identifier: GPL-2.0-only

package main

func runWovenSheet(c *caller) error {
	if err := addLiveChequerTiles(c, 0, "new"); err != nil {
		return err
	}
	if err := addLiveChequerTiles(c, 1, "join"); err != nil {
		return err
	}
	return c.checkVolumeTol("woven-sheet", 64*0.3*0.2*0.08, 0.001)
}
