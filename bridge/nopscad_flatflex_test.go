// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopFlatFlex(t *testing.T) {
	cs := freshPart(t)
	for _, p := range [][2]string{{"slotW", "11.8 mm"}, {"latchW", "17 mm"}, {"latchT", "1.4 mm"}, {"backW", "16 mm"}, {"midW", "12 mm"}} {
		addNopParam(t, cs, p[0], p[1])
	}
	addBoxFeature(t, cs, [][]float64{{-0.85, -0.27}, {0.85, -0.13}}, "latchW", "latchT", "1.2 mm", "new", "flat flex latch")
	addBoxFeature(t, cs, [][]float64{{-0.59, -0.32}, {0.59, -0.08}}, "slotW", "2.4 mm", "2 mm", "cut", "flat flex slot")
	addBoxFeature(t, cs, [][]float64{{-0.8, -0.27}, {0.8, 0.13}}, "backW", "4 mm", "2.5 mm", "join", "flat flex back")
	addBoxFeature(t, cs, [][]float64{{-0.6, 0.13}, {0.6, 0.29}}, "midW", "1.6 mm", "1.2 mm", "join", "flat flex mid")
	if got := partVolume(t, cs); got <= 0.1 {
		t.Errorf("flat_flex volume = %.6f, want assembled connector", got)
	}
}
