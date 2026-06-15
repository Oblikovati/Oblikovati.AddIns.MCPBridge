// SPDX-License-Identifier: GPL-2.0-only

package bridge

// addSectionArg is the mcp:input override for add_design_view_section (M12-F04): its wire DTO
// nests a types.SectionPlane whose Origin/Normal are flat [x,y,z] arrays the SDK schema rejects.
type addSectionArg struct {
	Rep   uint64 `json:"rep"`
	Plane struct {
		Origin  []float64 `json:"origin"`
		Normal  []float64 `json:"normal"`
		Flipped bool      `json:"flipped,omitempty"`
	} `json:"plane"`
}

// setFlexibleChildArg is the mcp:input override for set_flexible_child (M12-F06): the transform
// is a flat 16-cell array (types.Matrix's JSON), which the struct-derived schema would reject.
type setFlexibleChildArg struct {
	Occurrence uint64    `json:"occurrence"`
	Child      string    `json:"child"`
	Transform  []float64 `json:"transform"`
}
