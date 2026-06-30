// SPDX-License-Identifier: GPL-2.0-only

package bridge

import "oblikovati.org/api/wire"

// registerPanelTools registers the dockable-window value-edit tool.
//
// It is hand-registered rather than emitted by mcpgen ONLY because the generator is currently
// blocked by an unrelated api drift (the Attributes Set/Get/Delete methods delegate to *On variants
// that mcpgen can't map). Fold this into the generated surface once mcpgen is unblocked; the tool
// itself is generated normally from the api/client `mcp:tool set_panel_value` annotation.
func (s *Server) registerPanelTools() {
	addForward[wire.SetDockableWindowValueArgs](s, "set_panel_value",
		"Set one editable control of an add-in dockable window to a value, exactly as a user edit "+
			"would: the option text for a dropdown/combo, \"true\"/\"false\" for a checkbox, the number "+
			"for a value editor/slider, the text for a text box. The host updates the control and "+
			"notifies the owning add-in, which may react (e.g. switch the CAM simulator's View "+
			"dropdown to Path). Args: windowId, controlId, value.",
		wire.MethodDockableWindowsSetValue)
}
