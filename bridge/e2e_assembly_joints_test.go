// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api/wire"
)

// This file validates the M12-F02 assembly joint surface end to end over the bridge: each
// joint type added through its MCP tool against real box geometry, solved (joints and
// constraints share one solver), with the free component's remaining degrees of freedom
// asserted — the same DOF the engine's unit tests check, now over MCP → bridge → router →
// solver. Plus the DS-joint (imposed-motion) surface.

// solveDOF runs the combined assembly solve and returns the free component's DOF.
func solveDOF(t *testing.T, cs *mcp.ClientSession, free uint64) int {
	t.Helper()
	var health wire.AssemblyHealthResult
	callJSON(t, cs, "solve_assembly_constraints", nil, &health)
	return dofOf(health, free)
}

// TestEndToEndJointFamilyDOF drives each joint type over MCP and asserts the free component's
// remaining DOF — the F02 acceptance generalized to the whole family.
func TestEndToEndJointFamilyDOF(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		ref     func(k boxKeys) string // which geometry the joint origins use
		wantDOF int
	}{
		{"rigid", "add_rigid_joint", func(k boxKeys) string { return k.topZ }, 0},
		{"rotational", "add_rotational_joint", func(k boxKeys) string { return k.edge }, 1},
		{"slider", "add_slider_joint", func(k boxKeys) string { return k.topZ }, 1},
		{"cylindrical", "add_cylindrical_joint", func(k boxKeys) string { return k.edge }, 2},
		{"planar", "add_planar_joint", func(k boxKeys) string { return k.topZ }, 3},
		{"ball", "add_ball_joint", func(k boxKeys) string { return k.topZ }, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs, g, f, k := twoBoxes(t)
			key := tc.ref(k)
			var added wire.AssemblyJointResult
			callJSON(t, cs, tc.tool, map[string]any{"a": geomRef(g, key), "b": geomRef(f, key)}, &added)
			if added.Joint.Type != tc.name || added.Joint.DegreesOfFreedom != tc.wantDOF {
				t.Fatalf("added joint = %+v, want %s with %d DOF", added.Joint, tc.name, tc.wantDOF)
			}
			if got := solveDOF(t, cs, f); got != tc.wantDOF {
				t.Errorf("free component DOF after %s joint = %d, want %d", tc.name, got, tc.wantDOF)
			}
			var list wire.AssemblyJointsResult
			callJSON(t, cs, "list_assembly_joints", nil, &list)
			if len(list.Joints) != 1 {
				t.Errorf("list after %s = %d joints, want 1", tc.name, len(list.Joints))
			}
		})
	}
}

// TestEndToEndRotationalJointFlipLimitsDelete drives the mutate path: a rotational joint,
// flip, an angular limit, then delete.
func TestEndToEndRotationalJointFlipLimitsDelete(t *testing.T) {
	cs, g, f, k := twoBoxes(t)
	var added wire.AssemblyJointResult
	callJSON(t, cs, "add_rotational_joint", map[string]any{"a": geomRef(g, k.edge), "b": geomRef(f, k.edge)}, &added)

	var flipped wire.AssemblyJointResult
	callJSON(t, cs, "set_joint_flip", map[string]any{"id": added.Joint.ID, "flip": true}, &flipped)
	if !flipped.Joint.Flip {
		t.Errorf("set_joint_flip did not flip: %+v", flipped.Joint)
	}

	var limited wire.AssemblyJointResult
	callJSON(t, cs, "set_joint_limits", map[string]any{
		"id": added.Joint.ID, "limits": map[string]any{"hasAngularMax": true, "angularMax": 1.5},
	}, &limited)
	if limited.Joint.Limits == nil || limited.Joint.Limits.AngularMax != 1.5 {
		t.Errorf("set_joint_limits = %+v, want angular max 1.5", limited.Joint.Limits)
	}

	var afterDel wire.AssemblyJointsResult
	callJSON(t, cs, "delete_assembly_joint", map[string]any{"id": added.Joint.ID}, &afterDel)
	if len(afterDel.Joints) != 0 {
		t.Errorf("after delete = %+v, want empty set", afterDel.Joints)
	}
}

// TestEndToEndDSJoint drives the DS-joint surface: add a cylindrical DS joint, lock a DOF,
// and list.
func TestEndToEndDSJoint(t *testing.T) {
	cs, g, f, k := twoBoxes(t)
	var added wire.DSJointResult
	callJSON(t, cs, "add_ds_joint", map[string]any{"a": geomRef(g, k.edge), "b": geomRef(f, k.edge), "type": "cylindrical"}, &added)
	if len(added.Joint.DegreesOfFreedom) != 2 {
		t.Fatalf("DS cylindrical joint = %+v, want 2 DOF", added.Joint)
	}

	var locked wire.DSJointResult
	callJSON(t, cs, "set_ds_joint_imposed_motion", map[string]any{"id": added.Joint.ID, "dofIndex": 0, "imposedMotion": "locked"}, &locked)
	if locked.Joint.DegreesOfFreedom[0].ImposedMotion != "locked" {
		t.Errorf("DOF 0 imposed motion = %q, want locked", locked.Joint.DegreesOfFreedom[0].ImposedMotion)
	}

	var list wire.DSJointsResult
	callJSON(t, cs, "list_ds_joints", nil, &list)
	if len(list.Joints) != 1 {
		t.Errorf("DS joint list = %d, want 1", len(list.Joints))
	}
}
