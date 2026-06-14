// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopSmdTant(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-0.36, -0.21}, {0.36, 0.21}}, "7.2 mm", "4.2 mm", "2.4 mm", "new", "smd tantalum body")
	for _, x := range []float64{-0.41, 0.41} {
		addBoxFeature(t, cs, [][]float64{{x - 0.12, -0.17}, {x + 0.12, 0.17}}, "2.4 mm", "3.4 mm", "0.5 mm", "join", "smd tantalum lead")
	}
	addBoxFeature(t, cs, [][]float64{{-0.17, -0.21}, {0.17, 0.21}}, "3.4 mm", "4.2 mm", "1 mm", "cut", "smd tant lead gap")
	addBoxFeature(t, cs, [][]float64{{-0.31, -0.15}, {-0.23, 0.15}}, "0.8 mm", "3 mm", "0.1 mm", "join", "smd tant stripe")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("smd_tant volume = %.6f, want positive package", got)
	}
}
