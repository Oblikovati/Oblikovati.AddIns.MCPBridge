// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopE3dFan(t *testing.T) {
	cs := freshPart(t)
	addBridgeE3dDuct(t, cs)
	addBoxFeature(t, cs, [][]float64{{1.5, -1.5}, {2.5, 1.5}}, "10 mm", "30 mm", "3 mm", "join", "e3d fan frame")
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{2.0, 0}, "11 mm", "11 mm")
	applyCut(t, cs, s, 0, "e3d fan aperture")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("e3d_fan volume = %.6f, want positive", got)
	}
}
