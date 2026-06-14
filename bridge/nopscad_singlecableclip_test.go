// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopSingleCableClip(t *testing.T) {
	cs := freshPart(t)
	addBoxFeature(t, cs, [][]float64{{-0.8, -0.09}, {0.8, 0.09}}, "16 mm", "1.8 mm", "5 mm", "new", "cable clip foot")
	addBoxFeature(t, cs, [][]float64{{-0.2, -0.45}, {0.2, 0.45}}, "4 mm", "9 mm", "5 mm", "new", "cable clip post")
	s := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s, []float64{-0.55, 0.62}, "4.4 mm", "4.4 mm")
	applyNew(t, cs, s, 0, "5 mm", "cable clip loop")
	if healthy, reason := applyFeature(t, cs, "hull", map[string]any{}); !healthy {
		t.Fatalf("single cable clip hull unhealthy: %s", reason)
	}
	addBridgeSlotCut(t, cs, []float64{-0.45, 0.45}, []float64{-0.45, 0.05}, 0.18, "cable channel")
	sScrew := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, sScrew, []float64{0.45, 0.45}, "3 mm", "3 mm")
	applyCut(t, cs, sScrew, 0, "cable clip screw")
	if got := partVolume(t, cs); got <= 0 {
		t.Errorf("single_cable_clip volume = %.6f, want positive clip", got)
	}
}
