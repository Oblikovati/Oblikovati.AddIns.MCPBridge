// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"fmt"
	"math"
)

func addBoxFeature(c *caller, points [][]float64, widthExpr string, heightExpr string, distance string, operation string) error {
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, &sk)
	if c.err != nil {
		return c.err
	}
	sketchIndex := sk.SketchIndex
	addConstrainedRect(c, sketchIndex, points, widthExpr, heightExpr)
	if c.err != nil {
		return c.err
	}
	return c.applyFeature("extrude", map[string]any{"sketchIndex": sketchIndex, "profileIndex": 0, "distance": distance, "operation": operation})
}
func addConstrainedRect(c *caller, sketchIndex int, points [][]float64, widthExpr string, heightExpr string) {
	var rect struct {
		PointIDs []uint64 `json:"pointIds"`
	}
	c.json("add_sketch_entity", map[string]any{"sketchIndex": sketchIndex, "kind": "rectangle", "points": points}, &rect)
	if c.err != nil || len(rect.PointIDs) < 4 {
		c.err = fmt.Errorf("rectangle reply: %v (%v)", rect.PointIDs, c.err)
		return
	}
	bl, br, tr, tl := rect.PointIDs[0], rect.PointIDs[1], rect.PointIDs[2], rect.PointIDs[3]
	c.conAt(sketchIndex, "ground", bl)
	c.conAt(sketchIndex, "horizontal", bl, br)
	c.conAt(sketchIndex, "horizontal", tl, tr)
	c.conAt(sketchIndex, "vertical", bl, tl)
	c.conAt(sketchIndex, "vertical", br, tr)
	c.dimAt(sketchIndex, "distance", widthExpr, bl, br)
	c.dimAt(sketchIndex, "distance", heightExpr, bl, tl)
	if err := c.requireConstrainedAt(sketchIndex); err != nil {
		c.err = err
	}
}
func addConstrainedCircle(c *caller, sketchIndex int, center []float64, seedRadius string, radiusExpr string) {
	ids := c.ids(map[string]any{"sketchIndex": sketchIndex, "kind": "circle", "points": [][]float64{center}, "radius": seedRadius})
	if c.err != nil || len(ids) < 2 {
		c.err = fmt.Errorf("circle reply: %v (%v)", ids, c.err)
		return
	}
	c.conAt(sketchIndex, "ground", ids[1])
	c.dimAt(sketchIndex, "radius", radiusExpr, ids[0])
	if err := c.requireConstrainedAt(sketchIndex); err != nil {
		c.err = err
	}
}
func lightStripClipArea() float64 {
	const wall = 0.18
	const slot = 1.02
	const aperture = 0.60
	clipLength := slot + 2*wall
	clipWidth := 0.30 + 2*wall
	innerTop := clipWidth - 2*wall
	return polygonArea([][]float64{{-clipLength / 2, -wall}, {clipLength / 2, -wall}, {clipLength / 2, clipWidth - wall}, {aperture / 2, clipWidth - wall}, {aperture / 2, innerTop}, {slot / 2, innerTop}, {slot / 2, 0}, {-slot / 2, 0}, {-slot / 2, innerTop}, {-aperture / 2, innerTop}, {-aperture / 2, clipWidth - wall}, {-clipLength / 2, clipWidth - wall}})
}
func polygonArea(points [][]float64) float64 {
	var area float64
	for i := range points {
		j := (i + 1) % len(points)
		area += points[i][0] * points[j][1]
		area -= points[j][0] * points[i][1]
	}
	return math.Abs(area) / 2
}
func regularPolygon(sides int, radius float64, angleOffset float64) [][]float64 {
	points := make([][]float64, sides)
	for i := range sides {
		angle := angleOffset + 2*math.Pi*float64(i)/float64(sides)
		points[i] = []float64{radius * math.Cos(angle), radius * math.Sin(angle)}
	}
	return points
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
func extrudeProfilePart(c *caller, name string, points [][]float64, distance string, depthCM float64) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": 0, "kind": "polyline", "closed": true, "points": points}, nil)
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": distance, "operation": "new"}); err != nil {
		return err
	}
	return c.checkVolumeTol(name, polygonArea(points)*depthCM, 0.001)
}
func errVolume(name string, got, floor float64) error {
	return &volumeFloorError{name: name, got: got, floor: floor}
}

type volumeFloorError struct {
	name       string
	got, floor float64
}

func (e *volumeFloorError) Error() string {
	return e.name + " volume did not increase after join"
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
func addCouplingWithGrubs(c *caller) error {
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 0, []float64{0, 0}, "6 mm", "6 mm")
	addConstrainedCircle(c, 0, []float64{0, 0}, "2.5 mm", "2.5 mm")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 0, "profileIndex": 0, "distance": "20 mm", "operation": "new"}); err != nil {
		return err
	}
	idx := 1
	for _, g := range []struct {
		plane string
		z     float64
	}{{"YZ", 0.5}, {"XZ", -0.5}, {"YZ", -0.5}, {"XZ", 0.5}} {
		c.json("create_sketch", map[string]any{"plane": g.plane}, nil)
		addConstrainedCircle(c, idx, []float64{0, g.z}, "0.8 mm", "0.8 mm")
		if err := c.applyFeature("extrude", map[string]any{"sketchIndex": idx, "profileIndex": 0, "distance": "16 mm", "operation": "cut", "direction": "symmetric"}); err != nil {
			return err
		}
		idx++
	}
	return nil
}
func addLiveChequerTiles(c *caller, odd int, op string) error {
	first := op
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			px := -1.2 + 0.3*(2*float64(x)+float64((y+odd)%2))
			py := -0.8 + 0.2*float64(y)
			if px+0.3 > 1.2+1e-9 || py+0.2 > 0.8+1e-9 {
				continue
			}
			if err := addBoxFeature(c, [][]float64{{px, py}, {px + 0.3, py + 0.2}}, "3 mm", "2 mm", "0.8 mm", first); err != nil {
				return err
			}
			first = "join"
		}
	}
	return nil
}
func addLiveE3dDuct(c *caller) error {
	if err := addBoxFeature(c, [][]float64{{-0.8, -1.15}, {-0.75, 1.15}}, "0.5 mm", "23 mm", "26 mm", "new"); err != nil {
		return err
	}
	if err := addBoxFeature(c, [][]float64{{1.5, -1.5}, {1.55, 1.5}}, "0.5 mm", "30 mm", "30 mm", "new"); err != nil {
		return err
	}
	if err := c.applyFeature("hull", map[string]any{}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, nil)
	addConstrainedCircle(c, 2, []float64{0, 0}, "5.5 mm", "5.5 mm")
	if err := c.applyFeature("extrude", map[string]any{"sketchIndex": 2, "profileIndex": 0, "operation": "cut", "extent": "through-all"}); err != nil {
		return err
	}
	c.json("create_sketch", map[string]any{"plane": "YZ"}, nil)
	addConstrainedCircle(c, 3, []float64{0, 1.5}, "5.5 mm", "5.5 mm")
	return c.applyFeature("extrude", map[string]any{"sketchIndex": 3, "profileIndex": 0, "distance": "30 mm", "operation": "cut", "direction": "symmetric"})
}
func addLivePolylinePrism(c *caller, points [][]float64, depth, op string) error {
	var sk struct {
		SketchIndex int `json:"sketchIndex"`
	}
	c.json("create_sketch", map[string]any{"plane": "XY"}, &sk)
	c.json("add_sketch_entity", map[string]any{"sketchIndex": sk.SketchIndex, "kind": "polyline", "closed": true, "points": points}, nil)
	return c.applyFeature("extrude", map[string]any{"sketchIndex": sk.SketchIndex, "profileIndex": 0, "distance": depth, "operation": op})
}
func liveTearSlotPoints(radius, span float64, vertical bool) [][]float64 {
	a := liveTeardropPoints(-span/2, 0, radius, vertical)
	b := liveTeardropPoints(span/2, 0, radius, vertical)
	if vertical {
		a = liveTeardropPoints(0, -span/2, radius, vertical)
		b = liveTeardropPoints(0, span/2, radius, vertical)
	}
	points := append([][]float64{}, a...)
	for i := len(b) - 1; i >= 0; i-- {
		points = append(points, b[i])
	}
	return points
}
func liveTeardropPoints(cx, cy, r float64, vertical bool) [][]float64 {
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
	return append(points, []float64{cx + tipX, cy + tipY})
}
func addLiveSlotCut(c *caller, a, b []float64, radius float64) error {
	return addLivePolylinePrism(c, liveStadiumPoints(a, b, radius, 10), "20 mm", "cut")
}
func liveStadiumPoints(a, b []float64, radius float64, steps int) [][]float64 {
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
