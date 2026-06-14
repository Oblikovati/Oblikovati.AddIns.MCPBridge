// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"context"
	"embed"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// docsFS holds the static documentation served as MCP resources, so a connecting LLM
// can learn Oblikovati without prior knowledge.
//
//go:embed docs/getting-started.md docs/model.md
var docsFS embed.FS

// registerResources adds the self-teaching resources: static docs (embedded) and
// live schema resources backed by host methods (so they reflect the running app).
func (s *Server) registerResources() {
	s.addDocResource("oblikovati://docs/getting-started", "Getting started",
		"How to drive Oblikovati over this MCP server.", "docs/getting-started.md")
	s.addDocResource("oblikovati://docs/model", "Object model",
		"The Oblikovati document/parameter/sketch/feature/body object model.", "docs/model.md")
	s.addSchemaResource("oblikovati://schema/feature-kinds", "Feature kinds",
		"Live feature operations and their JSON argument schemas (read before add_feature).", "features.list")
	s.addSchemaResource("oblikovati://schema/commands", "Commands",
		"Live list of available commands (same data as list_commands).", "commands.list")
	s.addSchemaResource("oblikovati://schema/ribbon", "Ribbon",
		"The ribbon shown for the active document (ZeroDoc when none open): its tabs, panels, and controls.", "ribbon.list")
	s.addSchemaResource("oblikovati://schema/work-planes", "Work planes",
		"The active part's work planes (origin + user); errors when no part is active.", "workPlanes.list")
	s.addSchemaResource("oblikovati://schema/materials", "Materials",
		"The active document's materials.", "materials.list")
	s.addSchemaResource("oblikovati://schema/appearances", "Appearances",
		"The active document's appearances (visual styles).", "appearances.list")
	s.addSchemaResource("oblikovati://schema/themes", "Themes",
		"The available UI themes and which is active.", "theme.list")
	s.addSchemaResource("oblikovati://schema/sketches", "Sketches",
		"The active part's 2D sketches (index, name, plane, entity count, DOF).", "sketch.list")
	s.addSchemaResource("oblikovati://schema/sketches3d", "3D sketches",
		"The active part's 3D sketches.", "sketch3d.list")
	s.addSchemaResource("oblikovati://schema/display-modes", "Display modes",
		"The viewport display modes (visual styles) set_display_mode accepts.", "view.listDisplayModes")
	s.addSchemaResource("oblikovati://schema/lighting-styles", "Lighting styles",
		"The lighting styles set_lighting_style accepts.", "lighting.listStyles")
	s.addSchemaResource("oblikovati://schema/environment-presets", "Environment presets",
		"The built-in environment (sky/IBL) presets set_environment accepts.", "environment.listPresets")
	s.registerLogsResource()
}

// addDocResource serves an embedded markdown file at uri. A missing file is a build-
// time programming error (the embed would have failed), hence the panic.
func (s *Server) addDocResource(uri, name, desc, path string) {
	body, err := docsFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("bridge: embedded doc %q: %v", path, err))
	}
	s.mcp.AddResource(
		&mcp.Resource{URI: uri, Name: name, Description: desc, MIMEType: "text/markdown"},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return textResource(uri, "text/markdown", string(body)), nil
		})
}

// addSchemaResource serves the JSON result of a host method as a resource, so the
// schema reflects the live registry rather than a baked-in copy.
func (s *Server) addSchemaResource(uri, name, desc, method string) {
	s.mcp.AddResource(
		&mcp.Resource{URI: uri, Name: name, Description: desc, MIMEType: "application/json"},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			out, err := s.caller.Call(method, nil)
			if err != nil {
				return nil, err
			}
			return textResource(uri, "application/json", string(out)), nil
		})
}

// textResource wraps text as a single-content resource read result.
func textResource(uri, mime, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mime, Text: text}},
	}
}
