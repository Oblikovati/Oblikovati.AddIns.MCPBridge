// SPDX-License-Identifier: GPL-2.0-only

package rusthost

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"oblikovati.org/api"
	"oblikovati.org/api/types"
	"oblikovati.org/api/wire"
)

func TestRustMCPServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	client := connectRustHost(t, ctx)

	t.Run("API version", func(t *testing.T) {
		var version wire.ApplicationApiVersionResult
		callJSON(t, ctx, client, "application_api_version", nil, &version)
		if version.Version != api.Version {
			t.Fatalf("Rust host API = %q, want %q", version.Version, api.Version)
		}
	})

	t.Run("create and list documents", func(t *testing.T) {
		doc := createPart(t, ctx, client, "rust-mcp-documents.opd")
		var result wire.ListDocumentsResult
		callJSON(t, ctx, client, "list_documents", nil, &result)
		for _, open := range result.Documents {
			if open.ID == doc.ID && open.Name == doc.Name && open.Active {
				return
			}
		}
		t.Fatalf("active document %+v missing from %+v", doc, result.Documents)
	})

	t.Run("sketch and extrude", func(t *testing.T) {
		createPart(t, ctx, client, "rust-mcp-extrusion.opd")
		callTool(t, ctx, client, "add_parameter", wire.ParameterSetArgs{Name: "depth", Expression: "50 mm"})
		extrudeRectangle(t, ctx, client)
		assertSolidVolume(t, ctx, client, 60_000)
		callTool(t, ctx, client, "set_parameter", wire.ParameterSetArgs{Name: "depth", Expression: "60 mm"})
		assertSolidVolume(t, ctx, client, 72_000)
		verifyViewportCapture(t, ctx, client)
	})

	t.Run("typed model property in drawing", func(t *testing.T) {
		verifyTitleBlockProperty(t, ctx, client)
	})
}

func verifyTitleBlockProperty(t *testing.T, ctx context.Context, client *mcp.ClientSession) {
	t.Helper()
	part := createPart(t, ctx, client, "rust-mcp-title-source.opd")
	path := filepath.Join(t.TempDir(), "title-source.opd")
	callTool(t, ctx, client, "documents_save_as", wire.SaveDocumentAsArgs{
		Document: part.ID, NewFullDocumentName: path,
	})
	callTool(t, ctx, client, "create_document", wire.CreateDocumentArgs{Type: "drawing", Name: "Title block"})
	callTool(t, ctx, client, "drawing_set_model_reference", wire.SetModelReferenceArgs{
		FullDocumentName: path,
	})
	for _, number := range []string{"PN-42", "PN-43"} {
		callTool(t, ctx, client, "documents_set_property", wire.SetPropertyArgs{
			Document: part.ID, Set: "Design Tracking Properties", Name: "Part Number",
			Value: types.StringVariant(number),
		})
		var block wire.TitleBlockFieldsResult
		callJSON(t, ctx, client, "drawing_title_block_fields", wire.TitleBlockFieldsArgs{}, &block)
		assertPartNumber(t, block, number)
	}
}

func assertPartNumber(t *testing.T, block wire.TitleBlockFieldsResult, want string) {
	t.Helper()
	for _, field := range block.Fields {
		if field.Name == "Part Number" && field.Value == want &&
			field.Source == "Design Tracking Properties:Part Number" {
			return
		}
	}
	t.Fatalf("title block fields = %+v, want Part Number %q from the referenced model", block.Fields, want)
}

func createPart(t *testing.T, ctx context.Context, client *mcp.ClientSession, name string) wire.DocumentInfo {
	t.Helper()
	var doc wire.DocumentInfo
	callJSON(t, ctx, client, "create_document", wire.CreateDocumentArgs{Type: "part", Name: name}, &doc)
	if doc.Type != "part" || !doc.Active || doc.Name != name {
		t.Fatalf("create_document = %+v, want active part %q", doc, name)
	}
	return doc
}

func extrudeRectangle(t *testing.T, ctx context.Context, client *mcp.ClientSession) {
	t.Helper()
	var sketch wire.CreateSketchResult
	callJSON(t, ctx, client, "create_sketch", wire.CreateSketchArgs{Plane: "XY"}, &sketch)
	callTool(t, ctx, client, "add_sketch_entity", wire.AddSketchEntityArgs{
		SketchIndex: sketch.SketchIndex, Kind: "rectangle", Points: [][]float64{{0, 0}, {4, 3}},
	})

	args, err := json.Marshal(map[string]any{
		"sketchIndex": sketch.SketchIndex, "profileIndex": 0, "distance": "depth",
	})
	if err != nil {
		t.Fatal(err)
	}
	callTool(t, ctx, client, "add_feature", wire.AddFeatureArgs{Kind: "extrude", Args: args})
	var tree wire.ModelTreeResult
	callJSON(t, ctx, client, "get_model_tree", nil, &tree)
	if tree.Sketches != 1 || tree.Bodies != 1 || len(tree.Features) != 1 ||
		tree.Features[0].Kind != "extrude" || tree.Features[0].Health != "" {
		t.Fatalf("model tree = %+v, want one sketch and one healthy extruded solid", tree)
	}
}

func assertSolidVolume(t *testing.T, ctx context.Context, client *mcp.ClientSession, want float64) {
	t.Helper()
	var result wire.MassPropertiesResult
	callJSON(t, ctx, client, "analysis_mass_properties", nil, &result)
	if math.Abs(result.VolumeMm3-want) > 0.01 {
		t.Fatalf("solid volume = %g mm³, want %g mm³", result.VolumeMm3, want)
	}
}

func verifyViewportCapture(t *testing.T, ctx context.Context, client *mcp.ClientSession) {
	t.Helper()
	result := callTool(t, ctx, client, "capture_viewport", wire.CaptureViewportArgs{
		Path: filepath.Join(t.TempDir(), "extrusion.png"),
	})
	for _, item := range result.Content {
		img, ok := item.(*mcp.ImageContent)
		if !ok || img.MIMEType != "image/png" {
			continue
		}
		decoded, err := png.Decode(bytes.NewReader(img.Data))
		if err != nil {
			t.Fatalf("decode viewport PNG: %v", err)
		}
		if decoded.Bounds().Empty() {
			t.Fatal("viewport PNG is empty")
		}
		return
	}
	t.Fatal("capture_viewport returned no PNG image")
}

func callJSON(t *testing.T, ctx context.Context, client *mcp.ClientSession, name string, args, out any) {
	t.Helper()
	result := callTool(t, ctx, client, name, args)
	for _, item := range result.Content {
		if message, ok := item.(*mcp.TextContent); ok {
			if err := json.Unmarshal([]byte(message.Text), out); err != nil {
				t.Fatalf("%s returned invalid JSON %q: %v", name, message.Text, err)
			}
			return
		}
	}
	t.Fatalf("%s returned no text content", name)
}

func callTool(t *testing.T, ctx context.Context, client *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if result.IsError {
		for _, item := range result.Content {
			if message, ok := item.(*mcp.TextContent); ok {
				t.Fatalf("%s: %s", name, message.Text)
			}
		}
		t.Fatalf("%s returned an MCP error", name)
	}
	return result
}
