// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopVerticalTearslot(t *testing.T) {
	cs := freshPart(t)
	addBridgeTearSlot(t, cs, 0.35, 0.8, "5 mm", true)
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("vertical_tearslot volume = %.6f, want positive", got)
	}
}
