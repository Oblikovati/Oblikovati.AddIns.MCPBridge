// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"math"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addNopParam(t *testing.T, cs *mcp.ClientSession, name string, expression string) {
	t.Helper()
	callJSON(t, cs, "add_parameter", map[string]any{"name": name, "expression": expression}, nil)
}
func addConstrainedCircle(t *testing.T, cs *mcp.ClientSession, sketchIndex int, center []float64, seedRadius string, radiusExpr string) {
	t.Helper()
	ids := idsOf(t, cs, map[string]any{"sketchIndex": sketchIndex, "kind": "circle", "points": [][]float64{center}, "radius": seedRadius})
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "ground", "entities": []uint64{ids[1]}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIndex, "kind": "radius", "entities": []uint64{ids[0]}, "expression": radiusExpr}, nil)
	requireDOF(t, cs, sketchIndex)
}
func addConstrainedCornerRectangle(t *testing.T, cs *mcp.ClientSession, sketchIndex int, points [][]float64, widthExpr string, heightExpr string) {
	t.Helper()
	var r struct {
		EntityIDs []uint64 `json:"entityIds"`
		PointIDs  []uint64 `json:"pointIds"`
	}
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": sketchIndex, "kind": "rectangle", "points": points}, &r)
	if len(r.PointIDs) != 4 {
		t.Fatalf("rectangle returned %d point ids, want 4", len(r.PointIDs))
	}
	bl, br, tr, tl := r.PointIDs[0], r.PointIDs[1], r.PointIDs[2], r.PointIDs[3]
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "ground", "entities": []uint64{bl}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "horizontal", "entities": []uint64{bl, br}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "horizontal", "entities": []uint64{tl, tr}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "vertical", "entities": []uint64{bl, tl}}, nil)
	callJSON(t, cs, "add_sketch_constraint", map[string]any{"sketchIndex": sketchIndex, "kind": "vertical", "entities": []uint64{br, tr}}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIndex, "kind": "distance", "entities": []uint64{bl, br}, "expression": widthExpr}, nil)
	callJSON(t, cs, "add_sketch_dimension", map[string]any{"sketchIndex": sketchIndex, "kind": "distance", "entities": []uint64{bl, tl}, "expression": heightExpr}, nil)
	requireDOF(t, cs, sketchIndex)
}
func assertExtrudeVolume(t *testing.T, cs *mcp.ClientSession, sketchIndex int, profileIndex int, distance string, want float64, label string) {
	t.Helper()
	applyNew(t, cs, sketchIndex, profileIndex, distance, label)
	checkPartVolume(t, cs, want, 0.001, label)
}
func addBoxFeature(t *testing.T, cs *mcp.ClientSession, points [][]float64, widthExpr string, heightExpr string, distance string, operation string, label string) {
	t.Helper()
	s := addSketchOn(t, cs)
	addConstrainedCornerRectangle(t, cs, s, points, widthExpr, heightExpr)
	applyFeatureOp(t, cs, s, 0, distance, operation, label)
}
func applyNew(t *testing.T, cs *mcp.ClientSession, sketchIndex int, profileIndex int, distance string, label string) {
	t.Helper()
	applyFeatureOp(t, cs, sketchIndex, profileIndex, distance, "new", label)
}
func applyJoin(t *testing.T, cs *mcp.ClientSession, sketchIndex int, profileIndex int, distance string, label string) {
	t.Helper()
	applyFeatureOp(t, cs, sketchIndex, profileIndex, distance, "join", label)
}
func applyCut(t *testing.T, cs *mcp.ClientSession, sketchIndex int, profileIndex int, label string) {
	t.Helper()
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sketchIndex, "profileIndex": profileIndex, "operation": "cut", "extent": "through-all"}); !healthy {
		t.Fatalf("%s cut unhealthy: %s", label, reason)
	}
}
func applyFeatureOp(t *testing.T, cs *mcp.ClientSession, sketchIndex int, profileIndex int, distance string, operation string, label string) {
	t.Helper()
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sketchIndex, "profileIndex": profileIndex, "distance": distance, "operation": operation}); !healthy {
		t.Fatalf("%s extrude unhealthy: %s", label, reason)
	}
}
func checkPartVolume(t *testing.T, cs *mcp.ClientSession, want float64, tolerance float64, label string) {
	t.Helper()
	if got := partVolume(t, cs); math.Abs(got-want)/want > tolerance {
		t.Errorf("%s volume = %.6f cm^3, want ~%.6f", label, got, want)
	}
}
func sbrRailSection2D() [][]float64 {
	return [][]float64{{-0.55, 2.0}, {-0.8, 2.5}, {-2.0, 2.5}, {-2.0, 2.0}, {-1.025, 2.0}, {-0.4, 0.55}, {0.4, 0.55}, {1.025, 2.0}, {2.0, 2.0}, {2.0, 2.5}, {0.8, 2.5}, {0.55, 2.0}}
}
func semiEllipseFrame(a, b, thickness float64, steps int) [][]float64 {
	points := make([][]float64, 0, 2*steps+2)
	for i := 0; i <= steps; i++ {
		angle := math.Pi * float64(i) / float64(steps)
		points = append(points, []float64{(a + thickness) * math.Cos(angle), -(b + thickness) * math.Sin(angle)})
	}
	for i := steps; i >= 0; i-- {
		angle := math.Pi * float64(i) / float64(steps)
		points = append(points, []float64{a * math.Cos(angle), -b * math.Sin(angle)})
	}
	return points
}
func stadiumBandPoints2D(cx, cy, halfLength, halfWidth float64, steps int) [][]float64 {
	points := make([][]float64, 0, 2*steps+2)
	for i := 0; i <= steps; i++ {
		a := math.Pi/2 - math.Pi*float64(i)/float64(steps)
		points = append(points, []float64{cx + halfLength + halfWidth*math.Cos(a), cy + halfWidth*math.Sin(a)})
	}
	for i := 0; i <= steps; i++ {
		a := -math.Pi/2 - math.Pi*float64(i)/float64(steps)
		points = append(points, []float64{cx - halfLength + halfWidth*math.Cos(a), cy + halfWidth*math.Sin(a)})
	}
	return points
}
func ribbonGrommetProfile2D(length, height, radius float64, steps int) [][]float64 {
	points := [][]float64{{-length / 2, 0}, {length / 2, 0}, {length / 2, height - radius}}
	for i := 0; i <= steps; i++ {
		a := math.Pi * float64(i) / float64(steps)
		points = append(points, []float64{length/2 - radius + radius*math.Cos(a), height - radius + radius*math.Sin(a)})
	}
	points = append(points, []float64{-length / 2, height - radius})
	return points
}
func roundedCornerRectPoints2D(width, height, radius float64, steps int) [][]float64 {
	points := [][]float64{{0, 0}, {width, 0}, {width, height - radius}}
	for i := 0; i <= steps; i++ {
		a := math.Pi / 2 * float64(i) / float64(steps)
		points = append(points, []float64{width - radius + radius*math.Cos(a), height - radius + radius*math.Sin(a)})
	}
	points = append(points, []float64{0, height})
	return points
}
func addShaftCouplingBase(t *testing.T, cs *mcp.ClientSession) {
	t.Helper()
	s0 := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, s0, []float64{0, 0}, "6 mm", "6 mm")
	addConstrainedCircle(t, cs, s0, []float64{0, 0}, "2.5 mm", "2.5 mm")
	applyNew(t, cs, s0, 0, "20 mm", "grub coupling")
	for _, g := range []struct {
		plane string
		z     float64
	}{{"YZ", 0.5}, {"XZ", -0.5}, {"YZ", -0.5}, {"XZ", 0.5}} {
		s := addSketchOnPlane(t, cs, g.plane)
		addConstrainedCircle(t, cs, s, []float64{0, g.z}, "0.8 mm", "0.8 mm")
		applyCut(t, cs, s, 0, "grub screw")
	}
}
func addChequerTiles(t *testing.T, cs *mcp.ClientSession, odd int, op string) {
	t.Helper()
	first := op
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			px := -1.2 + 0.3*(2*float64(x)+float64((y+odd)%2))
			py := -0.8 + 0.2*float64(y)
			if px+0.3 > 1.2+1e-9 || py+0.2 > 0.8+1e-9 {
				continue
			}
			addBoxFeature(t, cs, [][]float64{{px, py}, {px + 0.3, py + 0.2}}, "3 mm", "2 mm", "0.8 mm", first, "sheet tile")
			first = "join"
		}
	}
}
func addSketchOnPlane(t *testing.T, cs *mcp.ClientSession, plane string) int {
	t.Helper()
	var out struct {
		SketchIndex int `json:"sketchIndex"`
	}
	callJSON(t, cs, "create_sketch", map[string]any{"plane": plane}, &out)
	return out.SketchIndex
}
func ternaryOp(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
func addBridgeTearSlot(t *testing.T, cs *mcp.ClientSession, radius, span float64, depth string, vertical bool) {
	t.Helper()
	points := bridgeTearSlotPoints(radius, span, vertical)
	addPolylinePrism(t, cs, points, depth, "new", "tearslot")
}
func bridgeTearSlotPoints(radius, span float64, vertical bool) [][]float64 {
	a := bridgeTeardropPoints(-span/2, 0, radius, vertical)
	b := bridgeTeardropPoints(span/2, 0, radius, vertical)
	if vertical {
		a = bridgeTeardropPoints(0, -span/2, radius, vertical)
		b = bridgeTeardropPoints(0, span/2, radius, vertical)
	}
	points := append([][]float64{}, a...)
	for i := len(b) - 1; i >= 0; i-- {
		points = append(points, b[i])
	}
	return points
}
func bridgeTeardropPoints(cx, cy, r float64, vertical bool) [][]float64 {
	points := make([][]float64, 0, 22)
	for i := 0; i <= 20; i++ {
		a := -5*math.Pi/4 + 3*math.Pi/2*float64(i)/20
		x, y := r*math.Cos(a), r*math.Sin(a)
		if vertical {
			x, y = -y, x
		}
		points = append(points, []float64{cx + x, cy + y})
	}
	tipX, tipY := 0.0, 1.35*r
	if vertical {
		tipX, tipY = -tipY, tipX
	}
	points = append(points, []float64{cx + tipX, cy + tipY})
	return points
}
func addPolylinePrism(t *testing.T, cs *mcp.ClientSession, points [][]float64, depth, op, label string) {
	t.Helper()
	s := addSketchOn(t, cs)
	callJSON(t, cs, "add_sketch_entity", map[string]any{"sketchIndex": s, "kind": "polyline", "closed": true, "points": points}, nil)
	applyFeatureOp(t, cs, s, 0, depth, op, label)
}
func addBridgeE3dDuct(t *testing.T, cs *mcp.ClientSession) {
	t.Helper()
	addBoxFeature(t, cs, [][]float64{{-0.8, -1.15}, {-0.75, 1.15}}, "0.5 mm", "23 mm", "26 mm", "new", "duct inlet")
	addBoxFeature(t, cs, [][]float64{{1.5, -1.5}, {1.55, 1.5}}, "0.5 mm", "30 mm", "30 mm", "new", "duct outlet")
	if healthy, reason := applyFeature(t, cs, "hull", map[string]any{}); !healthy {
		t.Fatalf("duct hull unhealthy: %s", reason)
	}
	sRad := addSketchOn(t, cs)
	addConstrainedCircle(t, cs, sRad, []float64{0, 0}, "5.5 mm", "5.5 mm")
	applyCut(t, cs, sRad, 0, "duct radial clearance")
	sCross := addSketchOnPlane(t, cs, "YZ")
	addConstrainedCircle(t, cs, sCross, []float64{0, 1.5}, "5.5 mm", "5.5 mm")
	if healthy, reason := applyFeature(t, cs, "extrude", map[string]any{"sketchIndex": sCross, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"}); !healthy {
		t.Fatalf("duct cross cut unhealthy: %s", reason)
	}
}
func addBridgeSlotCut(t *testing.T, cs *mcp.ClientSession, a, b []float64, radius float64, label string) {
	t.Helper()
	addPolylinePrism(t, cs, bridgeStadiumPoints(a, b, radius, 10), "20 mm", "cut", label)
}
func bridgeStadiumPoints(a, b []float64, radius float64, steps int) [][]float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	angle := math.Atan2(dy, dx)
	points := make([][]float64, 0, 2*(steps+1))
	for i := 0; i <= steps; i++ {
		theta := angle - math.Pi/2 + math.Pi*float64(i)/float64(steps)
		points = append(points, []float64{b[0] + radius*math.Cos(theta), b[1] + radius*math.Sin(theta)})
	}
	for i := 0; i <= steps; i++ {
		theta := angle + math.Pi/2 + math.Pi*float64(i)/float64(steps)
		points = append(points, []float64{a[0] + radius*math.Cos(theta), a[1] + radius*math.Sin(theta)})
	}
	return points
}
