// SPDX-License-Identifier: GPL-2.0-only

package main

func runLiveTearslot(c *caller, name, depth string, vertical bool) error {
	points := liveTearSlotPoints(0.35, 0.8, vertical)
	if err := addLivePolylinePrism(c, points, depth, "new"); err != nil {
		return err
	}
	if got := c.volume(); got <= 0 {
		return errVolume(name, got, 0)
	}
	return nil
}
