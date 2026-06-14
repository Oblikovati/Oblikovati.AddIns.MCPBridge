// SPDX-License-Identifier: GPL-2.0-only

package bridge

// Local tool inputs for the matrix-bearing assembly tools (mcp:input overrides). The host's wire
// DTOs carry the placement as a types.Matrix, whose JSON form is a flat 16-cell array — but the MCP
// SDK derives each tool's input schema from the Go struct shape, and a types.Matrix reflects as an
// object, not the array the model must send. So these mirror the wire fields with Transform as a
// plain []float64; the generated registration (mcpgen) re-marshals them to the same JSON the host's
// types.Matrix.UnmarshalJSON accepts. (The same local-input boundary noArgs and addFeatureArg use.)
type (
	placeComponentArg struct {
		Document  uint64    `json:"document"`
		Name      string    `json:"name"`
		Transform []float64 `json:"transform"`
	}
	placeComponentCopyArg struct {
		Source    uint64    `json:"source"`
		Name      string    `json:"name"`
		Transform []float64 `json:"transform"`
	}
	transformOccurrenceArg struct {
		ID        uint64    `json:"id"`
		Transform []float64 `json:"transform"`
	}
)
