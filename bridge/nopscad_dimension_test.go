// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopDimension(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-0.75, -0.015}, {0.75, 0.015}}, "15 mm", "0.3 mm", "0.3 mm", "new", "dimension line")
	addPolylinePrism(t, cs, [][]float64{{-0.7, -0.08}, {-0.7, 0.08}, {-0.95, 0}}, "0.3 mm", "join", "left dimension arrow")
	addPolylinePrism(t, cs, [][]float64{{0.7, -0.08}, {0.7, 0.08}, {0.95, 0}}, "0.3 mm", "join", "right dimension arrow")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("dimension volume = %.6f, want positive", got)
	}
}
