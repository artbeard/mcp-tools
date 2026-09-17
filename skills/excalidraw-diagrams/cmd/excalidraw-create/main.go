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
}
type connection struct {
	ID    string `json:"id"`
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
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
}
type roundness struct {
	Type int `json:"type"`
}
type boundElement struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
type binding struct {
	ElementID string  `json:"elementId"`
	Focus     float64 `json:"focus"`
	Gap       float64 `json:"gap"`
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
		arrow := arrowElement(arrowID, from, to)
		// An edge label is bound text of the arrow, not independent text at its
		// initial midpoint. Excalidraw will therefore move it with the arrow.
		if strings.TrimSpace(c.Label) != "" {
			labelID := "label-" + arrowID
			arrow.BoundElements = []boundElement{{ID: labelID, Type: "text"}}
			elements = append(elements, arrow, arrowLabelElement(labelID, arrowID, c.Label, from, to))
		} else {
			elements = append(elements, arrow)
		}
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
	return element{ID: id, Type: typ, X: x, Y: y, Width: width, Height: height, StrokeColor: "#1e1e1e", BackgroundColor: "#e3f2fd", FillStyle: "solid", StrokeWidth: 2, StrokeStyle: "solid", Roughness: 1, Opacity: 100, GroupIDs: []string{}, FrameID: nil, Index: "a0", Seed: seed(id), Version: 1, VersionNonce: seed(id + "-nonce"), IsDeleted: false, Link: nil, Locked: false}
}
func labelElement(id, container string, n node) element {
	textWidth := math.Min(n.Width-20, textWidth(n.Label))
	textHeight := textHeight(n.Label)
	return element{ID: id, Type: "text", X: n.XValue() + (n.Width-textWidth)/2, Y: n.YValue() + (n.Height-textHeight)/2, Width: textWidth, Height: textHeight, Angle: 0, StrokeColor: "#1e1e1e", BackgroundColor: "transparent", FillStyle: "solid", StrokeWidth: 2, StrokeStyle: "solid", Roughness: 0, Opacity: 100, GroupIDs: []string{}, FrameID: nil, Index: "a0", Seed: seed(id), Version: 1, VersionNonce: seed(id + "-nonce"), IsDeleted: false, Link: nil, Locked: false, Text: n.Label, FontSize: 20, FontFamily: 5, TextAlign: "center", VerticalAlign: "middle", ContainerID: &container, OriginalText: n.Label, AutoResize: true, LineHeight: 1.25}
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
	e := baseElement(id, "arrow", fx, fy, tx-fx, ty-fy)
	e.BackgroundColor = "transparent"
	e.FillStyle = "solid"
	e.Roughness = 0
	e.Points = [][]float64{{0, 0}, {tx - fx, ty - fy}}
	e.StartBinding = &binding{ElementID: "node-" + from.ID, Focus: 0, Gap: 0}
	e.EndBinding = &binding{ElementID: "node-" + to.ID, Focus: 0, Gap: 0}
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
func arrowLabelElement(id, arrowID, label string, from, to node) element {
	x := (from.XValue() + from.Width/2 + to.XValue() + to.Width/2) / 2
	y := (from.YValue() + from.Height/2 + to.YValue() + to.Height/2) / 2
	w, h := textWidth(label), textHeight(label)
	return element{ID: id, Type: "text", X: x - w/2, Y: y - h/2, Width: w, Height: h, Angle: 0, StrokeColor: "#1e1e1e", BackgroundColor: "transparent", FillStyle: "solid", StrokeWidth: 2, StrokeStyle: "solid", Roughness: 0, Opacity: 100, GroupIDs: []string{}, FrameID: nil, Index: "a0", Seed: seed(id), Version: 1, VersionNonce: seed(id + "-nonce"), IsDeleted: false, Link: nil, Locked: false, Text: label, FontSize: 16, FontFamily: 5, TextAlign: "center", VerticalAlign: "middle", ContainerID: &arrowID, OriginalText: label, AutoResize: true, LineHeight: 1.25}
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
