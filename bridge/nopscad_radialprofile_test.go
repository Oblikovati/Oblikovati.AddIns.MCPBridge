// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopRadialProfile(t *testing.T) {
	cs := freshPart(t)
	addPolylinePrism(t, cs, [][]float64{{0.16, 0}, {0.5, 0}, {0.5, 1.0}, {0.42, 1.16}, {0.22, 1.16}, {0.16, 1.0}}, "0.6 mm", "new", "radial profile")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("profile volume = %.6f, want positive radial half-profile", got)
	}
}
