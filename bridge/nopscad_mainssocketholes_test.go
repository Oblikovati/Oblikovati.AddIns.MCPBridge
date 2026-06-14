// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopMainsSocketHoles(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-1.8, -1.2}, {1.8, 1.2}}, "36 mm", "24 mm", "1.2 mm", "new", "mains panel")
	for _, x := range []float64{-1.25, 1.25} {
		s := addSketchOn(t, cs)
		addConstrainedCircle(t, cs, s, []float64{x, 0}, "1.6 mm", "1.6 mm")
		applyCut(t, cs, s, 0, "mains screw")
	}
	addBridgeSlotCut(t, cs, []float64{-0.45, 0}, []float64{0.45, 0}, 0.45, "mains aperture")
	sEarth := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, sEarth, []float64{-1.25, -0.75}, "2.2 mm", "2.2 mm")
	applyCut(t, cs, sEarth, 0, "mains earth")
	if got := partVolume(t, cs); got >= 3.6*2.4*0.12 {
		t.Errorf("mains_socket_holes volume = %.6f, want panel with cutouts", got)
	}
}
