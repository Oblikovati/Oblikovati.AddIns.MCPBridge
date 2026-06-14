// SPDX-License-Identifier: GPL-2.0-only

package bridge

// Local tool input for set_camera (mcp:input override). Its wire DTO carries Eye/Target as
// types.Point and Up as types.Vector, whose JSON form is a flat [x,y,z] array — but the MCP
// SDK derives the input schema from the Go struct, and those reflect as objects, so the
// validator rejects the array the model must send. This mirrors the wire fields with plain
// []float64 vectors; the generated registration re-marshals them to the same JSON the host's
// types.Point/Vector.UnmarshalJSON accepts. (Same boundary as the matrix args in
// tools_assembly.go.)
type setCameraArg struct {
	Document uint64    `json:"document,omitempty"`
	Eye      []float64 `json:"eye"`
	Target   []float64 `json:"target"`
	Up       []float64 `json:"up"`
	FOV      float64   `json:"fov"`
}
