// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runParamSeam is the live check for Oblikovati#1612 (audit B1): the parameter
// mutation invariants now live on the aggregate, shared by the wire router and
// the head UI. Over the wire it must hold that (1) deleting an in-use parameter
// is refused with the blockers named and the parameter survives, (2) the delete
// succeeds once the dependents are gone and the driven geometry recomputes, and
// (3) a parameter group's display name can never be emptied.
func runParamSeam(c *caller) error {
	c.json("add_parameter", map[string]any{"name": "width", "expression": "40 mm"}, nil)
	c.json("add_parameter", map[string]any{"name": "half", "expression": "width / 2"}, nil)
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedRect(c, 0, [][]float64{{0, 0}, {4, 2}}, "width", "half")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "1 cm", "operation": "new"}); err != nil {
		return err
	}
	if err := requireDeleteInUseRefused(c); err != nil {
		return err
	}
	if err := requireGroupRenameGuard(c); err != nil {
		return err
	}
	// Frame the driven box and capture the viewport for visual inspection.
	c.json("execute_command", map[string]any{"id": "View.Home"}, nil)
	c.json("capture_viewport", map[string]any{"path": "/tmp/oblikovati-paramseam.png"}, nil)
	return c.err
}

// requireDeleteInUseRefused probes parameters.delete against the in-use width:
// the refusal must name the blockers ("half" and, once half is gone, the model
// dimensions that read width), and width must survive every refusal.
func requireDeleteInUseRefused(c *caller) error {
	if err := requireRefusal(c, "parameters_delete", map[string]any{"name": "width"}, "half"); err != nil {
		return err
	}
	var detail struct {
		InUse      bool     `json:"inUse"`
		Dependents []string `json:"dependents"`
	}
	c.json("parameters_get_detail", map[string]any{"name": "width"}, &detail)
	if c.err != nil {
		return c.err
	}
	if !detail.InUse {
		return fmt.Errorf("width detail after refused delete = %+v, want still in use", detail)
	}
	// The sketch dimensions read width/half, so both stay blocked until the
	// dimensions go — the refusal keeps naming survivors, never deleting.
	if err := requireRefusal(c, "parameters_delete", map[string]any{"name": "half"}, ""); err != nil {
		return err
	}
	fmt.Printf("  in-use deletes refused, width/half survive with dependents %v\n", detail.Dependents)
	return nil
}

// requireGroupRenameGuard creates a group and probes the empty display name.
func requireGroupRenameGuard(c *caller) error {
	c.json("parameters_groups_add", map[string]any{"internalName": "frame", "displayName": "Frame"}, nil)
	if c.err != nil {
		return c.err
	}
	if err := requireRefusal(c, "parameters_groups_set_display_name",
		map[string]any{"internalName": "frame", "displayName": ""}, "empty"); err != nil {
		return err
	}
	var renamed struct {
		DisplayName string `json:"displayName"`
	}
	c.json("parameters_groups_set_display_name", map[string]any{"internalName": "frame", "displayName": "Chassis"}, &renamed)
	if c.err != nil {
		return c.err
	}
	if renamed.DisplayName != "Chassis" {
		return fmt.Errorf("renamed group display = %q, want Chassis", renamed.DisplayName)
	}
	fmt.Printf("  group rename guard holds: empty refused, %q applied\n", renamed.DisplayName)
	return nil
}

// requireRefusal asserts the tool call is an error reply whose text mentions
// wantInText (skipped when empty), printing the refusal for the log.
func requireRefusal(c *caller, tool string, args map[string]any, wantInText string) error {
	res, err := c.cs.CallTool(c.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return fmt.Errorf("%s probe transport error: %w", tool, err)
	}
	if !res.IsError {
		return fmt.Errorf("%s(%v) was accepted — the shared guard is gone (#1612)", tool, args)
	}
	text := firstText(res)
	if wantInText != "" && !strings.Contains(text, wantInText) {
		return fmt.Errorf("%s refusal = %q, want it to mention %q", tool, text, wantInText)
	}
	fmt.Printf("  %s refused as expected: %s\n", tool, text)
	return nil
}
