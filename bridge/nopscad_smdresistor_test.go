// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopSmdResistor(t *testing.T) {
	cs := freshPart(t)
	addNopParam(t, cs, "resZ", "1.2 mm")
	addNopParam(t, cs, "bodyX", "5.6 mm")
	addNopParam(t, cs, "bodyY", "2.5 mm")
	addNopParam(t, cs, "capX", "2.2 mm")
	addBoxFeature(t, cs, [][]float64{{-0.28, -0.125}, {0.28, 0.125}}, "bodyX", "bodyY", "resZ", "new", "smd resistor body")
	addBoxFeature(t, cs, [][]float64{{-0.50, -0.125}, {-0.28, 0.125}}, "capX", "bodyY", "resZ", "join", "smd resistor left cap")
	addBoxFeature(t, cs, [][]float64{{0.28, -0.125}, {0.50, 0.125}}, "capX", "bodyY", "resZ", "join", "smd resistor right cap")
	checkPartVolume(t, cs, 1.0*0.25*0.12, 0.001, "smd_resistor")
}
