// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopWovenSheet(t *testing.T) {
	cs := freshPart(t)
	addChequerTiles(t, cs, 0, "new")
	addChequerTiles(t, cs, 1, "join")
	checkPartVolume(t, cs, 64*0.3*0.2*0.08, 0.001, "woven_sheet")
}
