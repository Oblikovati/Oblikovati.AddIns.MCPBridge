// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"math"
)

func runGrubScrewPositions(c *caller) error {
	if err := addCouplingWithGrubs(c); err != nil {
		return err
	}
	if got := c.volume(); got >= math.Pi*(0.6*0.6-0.25*0.25)*2.0 {
		return errVolume("grubs", got, math.Pi*(0.6*0.6-0.25*0.25)*2.0)
	}
	return nil
}
