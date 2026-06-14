// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopE3dFanDuct(t *testing.T) {
	cs := freshPart(t)
	addBridgeE3dDuct(t, cs)
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("e3d_fan_duct volume = %.6f, want positive", got)
	}
}
