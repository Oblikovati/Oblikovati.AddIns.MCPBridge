// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"
)

func TestNopChequerboard(t *testing.T) {
	cs := freshPart(t)
	addChequerTiles(t, cs, 0, "new")
	checkPartVolume(t, cs, 32*0.3*0.2*0.08, 0.001, "chequerboard")
}
