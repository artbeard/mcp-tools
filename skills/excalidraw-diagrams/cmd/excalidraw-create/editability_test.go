package main

import (
	"math"
	"testing"
)

func TestCurrentBindingsReverseArrowAndMultilineLabel(t *testing.T) {
	result, err := buildScene(specification{Nodes: []node{
		{ID: "a", Label: "A", X: floatPointer(400), Y: floatPointer(0), Width: 200},
		{ID: "b", Label: "B", X: floatPointer(0), Y: floatPointer(0), Width: 100},
	}, Connections: []connection{{From: "a", To: "b", Label: "Да\nПроверено"}}})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, e := range result.Elements {
		if ids[e.Index] {
			t.Fatalf("duplicate index %s", e.Index)
		}
		ids[e.Index] = true
		if e.Type == "arrow" {
			if e.Width < 0 || e.Height < 0 || e.StartBinding.Mode != "orbit" || e.EndBinding.Mode != "orbit" {
				t.Fatalf("invalid geometry/binding: %#v", e)
			}
			if e.StartBinding.FixedPoint[0] != 0 || e.EndBinding.FixedPoint[0] != 1 {
				t.Fatalf("incorrect side bindings: %#v", e)
			}
		}
		if e.ID == "label-arrow-a-to-b" {
			if e.LabelPosition == nil || *e.LabelPosition != 0.5 || e.FontFamily != 2 || e.Height != 40 {
				t.Fatalf("incorrect bound label: %#v", e)
			}
			// Arrow runs from (400,40) to (100,40), independently of different block widths.
			if math.Abs(e.X+e.Width/2-250) > 0.001 || math.Abs(e.Y+e.Height/2-40) > 0.001 {
				t.Fatal("label is off the arrow")
			}
		}
	}
}

func TestOrthogonalRouteUsesFixedSegments(t *testing.T) {
	position := 0.25
	result, err := buildScene(specification{Nodes: []node{
		{ID: "a", Label: "A", X: floatPointer(0), Y: floatPointer(0)},
		{ID: "b", Label: "B", X: floatPointer(400), Y: floatPointer(200)},
	}, Connections: []connection{{From: "a", To: "b", Label: "Переход", StartSide: "right", EndSide: "left", Waypoints: [][2]float64{{300, 40}, {300, 240}}, LabelPosition: &position}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range result.Elements {
		if e.Type == "arrow" && (!e.Elbowed || len(e.FixedSegments) != 1 || e.FixedSegments[0].Index != 2) {
			t.Fatalf("route is not editable as an elbow arrow: %#v", e)
		}
		if e.Type == "text" && e.ID == "label-arrow-a-to-b" && (e.LabelPosition == nil || *e.LabelPosition != position) {
			t.Fatal("lost label position")
		}
	}
}

func TestIndicesRemainOrderedAcrossBase62Boundary(t *testing.T) {
	prev := ""
	for i := 0; i < 5000; i++ {
		next := elementIndex(i)
		if next <= prev {
			t.Fatalf("invalid order at %d: %q <= %q", i, next, prev)
		}
		prev = next
	}
}
