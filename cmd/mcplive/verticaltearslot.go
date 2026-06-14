// SPDX-License-Identifier: GPL-2.0-only

package main

func runVerticalTearslot(c *caller) error {
	return runLiveTearslot(c, "verticaltearslot", "5 mm", true)
}
