// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopVerticalTearslot2D(t *testing.T) {
	cs := freshPart(t)
	addBridgeTearSlot(t, cs, 0.35, 0.8, "0.8 mm", true)
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("vertical_tearslot_2d volume = %.6f, want positive", got)
	}
}
