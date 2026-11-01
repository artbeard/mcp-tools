// excalidraw-create creates a portable, editable Excalidraw scene from a small
// diagram specification. Geometry and element IDs are deterministic so it is
// suitable for generation by an agent and for review in Git.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultWidth  = 180.0
	defaultHeight = 80.0
	defaultGap    = 100.0
)

type specification struct {
	Nodes       []node       `json:"nodes"`
	Connections []connection `json:"connections"`
	Layout      layout       `json:"layout"`
}
type node struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Shape  string   `json:"shape"`
	X      *float64 `json:"x"`
	Y      *float64 `json:"y"`
	Width  float64  `json:"width"`
	Height float64  `json:"height"`
	Role   string   `json:"role,omitempty"`
}
type connection struct {
	ID            string       `json:"id"`
	From          string       `json:"from"`
	To            string       `json:"to"`
	Label         string       `json:"label"`
	StartSide     string       `json:"start_side,omitempty"`
	EndSide       string       `json:"end_side,omitempty"`
	Waypoints     [][2]float64 `json:"waypoints,omitempty"`
	LabelPosition *float64     `json:"label_position,omitempty"`
}
type layout struct {
	Direction string  `json:"direction"`
	Gap       float64 `json:"gap"`
	OriginX   float64 `json:"origin_x"`
	OriginY   float64 `json:"origin_y"`
}

type element struct {
	ID              string         `json:"id"`
	Type            string         `json:"type"`
	X               float64        `json:"x"`
	Y               float64        `json:"y"`
	Width           float64        `json:"width"`
	Height          float64        `json:"height"`
	Angle           float64        `json:"angle"`
	StrokeColor     string         `json:"strokeColor"`
	BackgroundColor string         `json:"backgroundColor"`
	FillStyle       string         `json:"fillStyle"`
	StrokeWidth     int            `json:"strokeWidth"`
	StrokeStyle     string         `json:"strokeStyle"`
	Roughness       int            `json:"roughness"`
	Opacity         int            `json:"opacity"`
	GroupIDs        []string       `json:"groupIds"`
	FrameID         *string        `json:"frameId"`
	Index           string         `json:"index"`
	Roundness       *roundness     `json:"roundness,omitempty"`
	Seed            int            `json:"seed"`
	Version         int            `json:"version"`
	VersionNonce    int            `json:"versionNonce"`
	IsDeleted       bool           `json:"isDeleted"`
	BoundElements   []boundElement `json:"boundElements,omitempty"`
	Link            *string        `json:"link"`
	Locked          bool           `json:"locked"`
	Text            string         `json:"text,omitempty"`
	FontSize        float64        `json:"fontSize,omitempty"`
	FontFamily      int            `json:"fontFamily,omitempty"`
	TextAlign       string         `json:"textAlign,omitempty"`
	VerticalAlign   string         `json:"verticalAlign,omitempty"`
	ContainerID     *string        `json:"containerId,omitempty"`
	OriginalText    string         `json:"originalText,omitempty"`
	AutoResize      bool           `json:"autoResize,omitempty"`
	LineHeight      float64        `json:"lineHeight,omitempty"`
	Points          [][]float64    `json:"points,omitempty"`
	StartBinding    *binding       `json:"startBinding,omitempty"`
	EndBinding      *binding       `json:"endBinding,omitempty"`
	StartArrowhead  *string        `json:"startArrowhead"`
	EndArrowhead    *string        `json:"endArrowhead"`
	Elbowed         bool           `json:"elbowed,omitempty"`
	FixedSegments   []fixedSegment `json:"fixedSegments,omitempty"`
	LabelPosition   *float64       `json:"labelPosition,omitempty"`
}
type roundness struct {
	Type int `json:"type"`
}
type boundElement struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
type binding struct {
	ElementID  string     `json:"elementId"`
	Mode       string     `json:"mode"`
	FixedPoint [2]float64 `json:"fixedPoint"`
}
type fixedSegment struct {
	Start [2]float64 `json:"start"`
	End   [2]float64 `json:"end"`
	Index int        `json:"index"`
}
type scene struct {
	Type     string         `json:"type"`
	Version  int            `json:"version"`
	Source   string         `json:"source"`
	Elements []element      `json:"elements"`
	AppState map[string]any `json:"appState"`
	Files    map[string]any `json:"files"`
}

func main() {
	input := flag.String("input", "", "path to diagram specification JSON")
	output := flag.String("output", "", "path to a new .excalidraw file")
	flag.Parse()
	if err := run(*input, *output); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(inputPath, outputPath string) error {
	if inputPath == "" || outputPath == "" {
		return errors.New("--input and --output are required")
	}
	if filepath.Clean(inputPath) == filepath.Clean(outputPath) {
		return errors.New("output must differ from input")
	}
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("output already exists: %s", outputPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	var spec specification
	if err := json.Unmarshal(raw, &spec); err != nil {
		return fmt.Errorf("input JSON: %w", err)
	}
	result, err := buildScene(spec)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(data, '\n'), 0644)
}

func buildScene(spec specification) (scene, error) {
	if len(spec.Nodes) == 0 {
		return scene{}, errors.New("nodes must not be empty")
	}
	if spec.Layout.Direction == "" {
		spec.Layout.Direction = "horizontal"
	}
	if spec.Layout.Direction != "horizontal" && spec.Layout.Direction != "vertical" {
		return scene{}, errors.New("layout.direction must be horizontal or vertical")
	}
	if spec.Layout.Gap == 0 {
		spec.Layout.Gap = defaultGap
	}
	if spec.Layout.Gap < 0 {
		return scene{}, errors.New("layout.gap must be non-negative")
	}
	nodes := make(map[string]node, len(spec.Nodes))
	for i := range spec.Nodes {
		n := &spec.Nodes[i]
		n.ID, n.Label = strings.TrimSpace(n.ID), strings.TrimSpace(n.Label)
		if n.ID == "" || n.Label == "" {
			return scene{}, errors.New("each node needs non-empty id and label")
		}
		if _, exists := nodes[n.ID]; exists {
			return scene{}, fmt.Errorf("duplicate node id %q", n.ID)
		}
		if n.Shape == "" {
			n.Shape = "rectangle"
		}
		if n.Shape != "rectangle" && n.Shape != "ellipse" && n.Shape != "diamond" {
			return scene{}, fmt.Errorf("node %q: shape must be rectangle, ellipse, or diamond", n.ID)
		}
		switch n.Role {
		case "", "front", "backend", "mixed", "external", "note", "decision":
		default:
			return scene{}, fmt.Errorf("node %q: unknown role %q", n.ID, n.Role)
		}
		if n.Width == 0 {
			n.Width = defaultWidth
		}
		if n.Height == 0 {
			n.Height = defaultHeight
		}
		if n.Width <= 0 || n.Height <= 0 {
			return scene{}, fmt.Errorf("node %q: width and height must be positive", n.ID)
		}
		nodes[n.ID] = *n
	}
	placeNodes(spec.Nodes, spec.Layout)
	for _, n := range spec.Nodes {
		nodes[n.ID] = n
	}

	elements := make([]element, 0, len(spec.Nodes)*2+len(spec.Connections)*2)
	positions := make(map[string]node, len(nodes))
	for _, n := range spec.Nodes {
		positions[n.ID] = n
		shapeID, textID := "node-"+n.ID, "label-"+n.ID
		shape := baseElement(shapeID, n.Shape, n.XValue(), n.YValue(), n.Width, n.Height)
		if n.Shape == "rectangle" {
			shape.Roundness = &roundness{Type: 3}
		}
		applyNodeStyle(&shape, n)
		shape.BoundElements = []boundElement{{ID: textID, Type: "text"}}
		elements = append(elements, shape, labelElement(textID, shapeID, n))
	}
	usedConnections := map[string]bool{}
	for i, c := range spec.Connections {
		c.ID, c.From, c.To = strings.TrimSpace(c.ID), strings.TrimSpace(c.From), strings.TrimSpace(c.To)
		if c.From == "" || c.To == "" {
			return scene{}, fmt.Errorf("connection %d needs from and to", i)
		}
		from, okFrom := positions[c.From]
		to, okTo := positions[c.To]
		if !okFrom || !okTo {
			return scene{}, fmt.Errorf("connection %d references an unknown node", i)
		}
		if c.ID == "" {
			c.ID = fmt.Sprintf("%s-to-%s", c.From, c.To)
		}
		if usedConnections[c.ID] {
			return scene{}, fmt.Errorf("duplicate connection id %q", c.ID)
		}
		usedConnections[c.ID] = true
		arrowID := "arrow-" + c.ID
		// The reciprocal boundElements entries are required in addition to
		// arrow bindings: Excalidraw then preserves the attachment on moves.
		for j := range elements {
			if elements[j].ID == "node-"+c.From || elements[j].ID == "node-"+c.To {
				elements[j].BoundElements = append(elements[j].BoundElements, boundElement{ID: arrowID, Type: "arrow"})
			}
		}
		for _, side := range []string{c.StartSide, c.EndSide} {
			if side != "" && side != "top" && side != "bottom" && side != "left" && side != "right" {
				return scene{}, fmt.Errorf("connection %q: invalid side %q", c.ID, side)
			}
		}
		if c.LabelPosition != nil && (*c.LabelPosition < 0 || *c.LabelPosition > 1) {
			return scene{}, fmt.Errorf("connection %q: label_position must be between 0 and 1", c.ID)
		}
		arrow := arrowElement(arrowID, from, to)
		routeArrow(&arrow, from, to, c)
		// An edge label is bound text of the arrow, not independent text at its
		// initial midpoint. Excalidraw will therefore move it with the arrow.
		if strings.TrimSpace(c.Label) != "" {
			labelID := "label-" + arrowID
			arrow.BoundElements = []boundElement{{ID: labelID, Type: "text"}}
			elements = append(elements, arrow, arrowLabelElement(labelID, arrow, c.Label, c.LabelPosition))
		} else {
			elements = append(elements, arrow)
		}
	}
	for i := range elements {
		elements[i].Index = elementIndex(i)
	}
	return scene{Type: "excalidraw", Version: 2, Source: "mcp-tools/excalidraw-create", Elements: elements, AppState: map[string]any{"viewBackgroundColor": "#ffffff", "gridSize": nil}, Files: map[string]any{}}, nil
}

func placeNodes(nodes []node, l layout) {
	x, y := l.OriginX, l.OriginY
	for i := range nodes {
		if nodes[i].X == nil {
			v := x
			nodes[i].X = &v
		}
		if nodes[i].Y == nil {
			v := y
			nodes[i].Y = &v
		}
		if l.Direction == "horizontal" {
			x += nodes[i].Width + l.Gap
		} else {
			y += nodes[i].Height + l.Gap
		}
	}
}
func (n node) XValue() float64 { return *n.X }
func (n node) YValue() float64 { return *n.Y }
func baseElement(id, typ string, x, y, width, height float64) element {
	return element{ID: id, Type: typ, X: x, Y: y, Width: width, Height: height, StrokeColor: "#1d4ed8", BackgroundColor: "#eff6ff", FillStyle: "solid", StrokeWidth: 2, StrokeStyle: "solid", Roughness: 0, Opacity: 100, GroupIDs: []string{}, FrameID: nil, Index: "a0", Seed: seed(id), Version: 1, VersionNonce: seed(id + "-nonce"), IsDeleted: false, Link: nil, Locked: false}
}
func labelElement(id, container string, n node) element {
	textWidth := math.Min(n.Width-20, textWidth(n.Label))
	textHeight := textHeight(n.Label)
	return element{ID: id, Type: "text", X: n.XValue() + (n.Width-textWidth)/2, Y: n.YValue() + (n.Height-textHeight)/2, Width: textWidth, Height: textHeight, Angle: 0, StrokeColor: "#0f172a", BackgroundColor: "transparent", FillStyle: "solid", StrokeWidth: 2, StrokeStyle: "solid", Roughness: 0, Opacity: 100, GroupIDs: []string{}, FrameID: nil, Index: "a0", Seed: seed(id), Version: 1, VersionNonce: seed(id + "-nonce"), IsDeleted: false, Link: nil, Locked: false, Text: n.Label, FontSize: 20, FontFamily: 2, TextAlign: "center", VerticalAlign: "middle", ContainerID: &container, OriginalText: n.Label, AutoResize: true, LineHeight: 1.25}
}
func arrowElement(id string, from, to node) element {
	fromCenterX, fromCenterY := from.XValue()+from.Width/2, from.YValue()+from.Height/2
	toCenterX, toCenterY := to.XValue()+to.Width/2, to.YValue()+to.Height/2
	dx, dy := toCenterX-fromCenterX, toCenterY-fromCenterY
	if dx == 0 && dy == 0 {
		// Validation normally prevents this case; keep the element valid if an
		// imported specification nevertheless puts two nodes at one point.
		dx = 1
	}
	fromOffsetX, fromOffsetY := boundaryOffset(from, dx, dy)
	toOffsetX, toOffsetY := boundaryOffset(to, -dx, -dy)
	fx, fy := fromCenterX+fromOffsetX, fromCenterY+fromOffsetY
	tx, ty := toCenterX+toOffsetX, toCenterY+toOffsetY
	end := "arrow"
	e := baseElement(id, "arrow", fx, fy, math.Abs(tx-fx), math.Abs(ty-fy))
	e.StrokeColor = "#475569"
	e.BackgroundColor = "transparent"
	e.FillStyle = "solid"
	e.Roughness = 0
	e.Points = [][]float64{{0, 0}, {tx - fx, ty - fy}}
	e.StartBinding = pointBinding(from, fx, fy)
	e.EndBinding = pointBinding(to, tx, ty)
	e.EndArrowhead = &end
	return e
}

// boundaryOffset returns the point at which a ray from the node centre meets
// its visual boundary. Excalidraw points are absolute for the first point and
// relative thereafter, so starting there prevents a newly opened scene from
// showing arrows through the middle of the objects.
func boundaryOffset(n node, dx, dy float64) (float64, float64) {
	halfWidth, halfHeight := n.Width/2, n.Height/2
	var scale float64
	switch n.Shape {
	case "ellipse":
		scale = 1 / math.Sqrt((dx*dx)/(halfWidth*halfWidth)+(dy*dy)/(halfHeight*halfHeight))
	case "diamond":
		scale = 1 / (math.Abs(dx)/halfWidth + math.Abs(dy)/halfHeight)
	default: // rectangle
		scale = math.Min(extentOverAbs(halfWidth, dx), extentOverAbs(halfHeight, dy))
	}
	return dx * scale, dy * scale
}

func extentOverAbs(extent, value float64) float64 {
	if value == 0 {
		return math.Inf(1)
	}
	return extent / math.Abs(value)
}
func arrowLabelElement(id string, arrow element, label string, position *float64) element {
	if position == nil {
		value := 0.5
		position = &value
	}
	x, y := pointAlongArrow(arrow, *position)
	w, h := textWidth(label)*16/20, textHeight(label)*16/20
	return element{ID: id, Type: "text", X: x - w/2, Y: y - h/2, Width: w, Height: h, StrokeColor: "#0f172a", BackgroundColor: "transparent", FillStyle: "solid", StrokeWidth: 2, StrokeStyle: "solid", Opacity: 100, GroupIDs: []string{}, Seed: seed(id), Version: 1, VersionNonce: seed(id + "-nonce"), Text: label, FontSize: 16, FontFamily: 2, TextAlign: "center", VerticalAlign: "middle", ContainerID: &arrow.ID, OriginalText: label, AutoResize: true, LineHeight: 1.25, LabelPosition: position}
}

func pointBinding(n node, x, y float64) *binding {
	ratios := [2]float64{(x - n.XValue()) / n.Width, (y - n.YValue()) / n.Height}
	for i, value := range ratios {
		if math.Abs(value-0.5) < 0.0001 {
			ratios[i] = 0.5001
		}
	}
	return &binding{ElementID: "node-" + n.ID, Mode: "orbit", FixedPoint: ratios}
}

func sidePoint(n node, side string, fallback [2]float64) [2]float64 {
	switch side {
	case "top":
		return [2]float64{n.XValue() + n.Width/2, n.YValue()}
	case "bottom":
		return [2]float64{n.XValue() + n.Width/2, n.YValue() + n.Height}
	case "left":
		return [2]float64{n.XValue(), n.YValue() + n.Height/2}
	case "right":
		return [2]float64{n.XValue() + n.Width, n.YValue() + n.Height/2}
	}
	return fallback
}

func routeArrow(e *element, from, to node, c connection) {
	start := sidePoint(from, c.StartSide, [2]float64{e.X, e.Y})
	end := sidePoint(to, c.EndSide, [2]float64{e.X + e.Points[1][0], e.Y + e.Points[1][1]})
	absolute := append([][2]float64{start}, c.Waypoints...)
	absolute = append(absolute, end)
	e.X, e.Y = start[0], start[1]
	e.Points = nil
	minX, maxX, minY, maxY := start[0], start[0], start[1], start[1]
	orthogonal := len(absolute) > 2
	for i, p := range absolute {
		e.Points = append(e.Points, []float64{p[0] - start[0], p[1] - start[1]})
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
		if i > 0 && p[0] != absolute[i-1][0] && p[1] != absolute[i-1][1] {
			orthogonal = false
		}
	}
	e.Width, e.Height = maxX-minX, maxY-minY
	e.StartBinding, e.EndBinding = pointBinding(from, start[0], start[1]), pointBinding(to, end[0], end[1])
	e.Elbowed = orthogonal
	if orthogonal {
		for i := 1; i < len(e.Points)-2; i++ {
			e.FixedSegments = append(e.FixedSegments, fixedSegment{Start: [2]float64{e.Points[i][0], e.Points[i][1]}, End: [2]float64{e.Points[i+1][0], e.Points[i+1][1]}, Index: i + 1})
		}
	}
}

func pointAlongArrow(e element, position float64) (float64, float64) {
	total := 0.0
	for i := 1; i < len(e.Points); i++ {
		total += math.Hypot(e.Points[i][0]-e.Points[i-1][0], e.Points[i][1]-e.Points[i-1][1])
	}
	remaining := total * position
	for i := 1; i < len(e.Points); i++ {
		a, b := e.Points[i-1], e.Points[i]
		length := math.Hypot(b[0]-a[0], b[1]-a[1])
		if length > 0 && (remaining <= length || i == len(e.Points)-1) {
			ratio := remaining / length
			return e.X + a[0] + (b[0]-a[0])*ratio, e.Y + a[1] + (b[1]-a[1])*ratio
		}
		remaining -= length
	}
	return e.X, e.Y
}

func applyNodeStyle(e *element, n node) {
	role := n.Role
	if role == "" && n.Shape == "diamond" {
		role = "decision"
	}
	switch role {
	case "backend":
		e.StrokeColor, e.BackgroundColor = "#15803d", "#f0fdf4"
	case "mixed":
		e.StrokeColor, e.BackgroundColor = "#0369a1", "#e0f2fe"
	case "external":
		e.StrokeColor, e.BackgroundColor, e.StrokeStyle = "#64748b", "#f1f5f9", "dashed"
	case "note":
		e.StrokeColor, e.BackgroundColor = "#64748b", "#f8fafc"
	case "decision":
		e.StrokeColor, e.BackgroundColor = "#b45309", "#fffbeb"
	}
}

// Sequential fractional indices, with a growing integer part after every base-62 boundary.
func elementIndex(i int) string {
	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	length, capacity := 1, 62
	for i >= capacity {
		i -= capacity
		length++
		capacity *= 62
	}
	value := make([]byte, length)
	for j := length - 1; j >= 0; j-- {
		value[j] = digits[i%62]
		i /= 62
	}
	return string(rune('a'+length-1)) + string(value)
}
func textWidth(text string) float64 {
	max := 1
	for _, line := range strings.Split(text, "\n") {
		if len([]rune(line)) > max {
			max = len([]rune(line))
		}
	}
	return float64(max) * 11.2
}
func textHeight(text string) float64 { return float64(len(strings.Split(text, "\n"))) * 25 }
func seed(value string) int {
	// FNV-1a expressed locally to avoid random IDs and external dependencies.
	var h uint32 = 2166136261
	for _, b := range []byte(value) {
		h ^= uint32(b)
		h *= 16777619
	}
	return int(h & 0x7fffffff)
}
