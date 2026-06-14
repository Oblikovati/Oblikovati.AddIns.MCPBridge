// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"
)

func TestNopGrubScrewPositions(t *testing.T) {
	cs := freshPart(t)
	addShaftCouplingBase(t, cs)
	if got := partVolume(t, cs); got >= math.Pi*(0.6*0.6-0.25*0.25)*2.0 {
		t.Errorf("grub_screw_positions volume = %.6f, want below uncut coupling", got)
	}
}
