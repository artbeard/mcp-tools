package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatesBoundArrowOnlyForDeclaredConnection(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "diagram.json"), filepath.Join(dir, "diagram.excalidraw")
	if err := os.WriteFile(in, []byte(`{"nodes":[{"id":"client","label":"Client"},{"id":"api","label":"API"}],"connections":[{"from":"client","to":"api","label":"HTTPS"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(in, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got scene
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	var arrow, arrowLabel *element
	bound := map[string]map[string]bool{"node-client": {}, "node-api": {}}
	for i := range got.Elements {
		e := &got.Elements[i]
		if e.Type == "arrow" {
			arrow = e
		}
		if e.ID == "label-arrow-client-to-api" {
			arrowLabel = e
		}
		if _, tracked := bound[e.ID]; tracked {
			for _, b := range e.BoundElements {
				bound[e.ID][b.ID] = true
			}
		}
	}
	if arrow == nil || arrow.StartBinding == nil || arrow.EndBinding == nil {
		t.Fatal("connection did not create a bound arrow")
	}
	if arrow.StartBinding.ElementID != "node-client" || arrow.EndBinding.ElementID != "node-api" || !bound["node-client"][arrow.ID] || !bound["node-api"][arrow.ID] {
		t.Fatalf("unexpected bindings: %#v / %#v", arrow.StartBinding, arrow.EndBinding)
	}
	if arrowLabel == nil || arrowLabel.ContainerID == nil || *arrowLabel.ContainerID != arrow.ID || len(arrow.BoundElements) != 1 || arrow.BoundElements[0].ID != arrowLabel.ID {
		t.Fatalf("arrow label is not bound: arrow=%#v label=%#v", arrow.BoundElements, arrowLabel)
	}
}

func TestRejectsUnknownConnectionNode(t *testing.T) {
	_, err := buildScene(specification{Nodes: []node{{ID: "a", Label: "A"}}, Connections: []connection{{From: "a", To: "missing"}}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestArrowTouchesRectangleBoundaries(t *testing.T) {
	result, err := buildScene(specification{Nodes: []node{
		{ID: "left", Label: "Left", X: floatPointer(0), Y: floatPointer(0)},
		{ID: "right", Label: "Right", X: floatPointer(400), Y: floatPointer(0)},
	}, Connections: []connection{{From: "left", To: "right"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, element := range result.Elements {
		if element.ID != "arrow-left-to-right" {
			continue
		}
		if element.X != 180 || element.Y != 40 || element.X+element.Points[1][0] != 400 || element.Y+element.Points[1][1] != 40 {
			t.Fatalf("arrow is not on boundaries: x=%v y=%v points=%v", element.X, element.Y, element.Points)
		}
		return
	}
	t.Fatal("arrow not found")
}

func floatPointer(value float64) *float64 { return &value }
