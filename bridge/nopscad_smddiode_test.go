// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopSmdDiode(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-0.23, -0.14}, {0.23, 0.14}}, "4.6 mm", "2.8 mm", "1.6 mm", "new", "smd diode body")
	for _, x := range []float64{-0.26, 0.26} {
		addBoxFeature(t, cs, [][]float64{{x - 0.09, -0.12}, {x + 0.09, 0.12}}, "1.8 mm", "2.4 mm", "0.4 mm", "join", "smd diode lead")
	}
	addBoxFeature(t, cs, [][]float64{{-0.11, -0.14}, {0.11, 0.14}}, "2.2 mm", "2.8 mm", "0.8 mm", "cut", "smd diode lead gap")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("smd_diode volume = %.6f, want positive package", got)
	}
}
