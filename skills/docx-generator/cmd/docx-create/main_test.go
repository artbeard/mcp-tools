package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreatesWord2024BaseDocument(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.json")
	outputPath := filepath.Join(dir, "result.docx")
	if err := os.WriteFile(inputPath, []byte(`{"title":"Отчёт","paragraphs":[{"text":"A & B"},{"text":"Раздел","style":"Heading1"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(inputPath, outputPath, ""); err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, outputPath)
	if !strings.Contains(string(entries["word/styles.xml"]), `w:ascii="Aptos"`) {
		t.Fatal("base document does not use Aptos")
	}
	doc := string(entries["word/document.xml"])
	if !strings.Contains(doc, "A &amp; B") || !strings.Contains(doc, `w:pStyle w:val="Heading1"`) {
		t.Fatalf("unexpected document XML: %s", doc)
	}
}

func TestKeepsTemplateStyles(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.docx")
	if err := writeDocx(templatePath, map[string][]byte{
		"[Content_Types].xml":   []byte("types"),
		"word/styles.xml":       []byte("template-styles"),
		"word/document.xml":     []byte(`<w:document><w:body>old-document<w:sectPr><w:pgSz w:w="777"/></w:sectPr></w:body></w:document>`),
		"word/theme/theme1.xml": []byte("template-theme"),
		"customXml/item1.xml":   []byte("custom-data"),
	}); err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "input.json")
	outputPath := filepath.Join(dir, "result.docx")
	if err := os.WriteFile(inputPath, []byte(`{"paragraphs":[{"text":"Новый текст"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(inputPath, outputPath, templatePath); err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, outputPath)
	if string(entries["word/styles.xml"]) != "template-styles" || string(entries["word/theme/theme1.xml"]) != "template-theme" {
		t.Fatal("template formatting parts were not preserved")
	}
	if strings.Contains(string(entries["word/document.xml"]), "old-document") {
		t.Fatal("template body was not replaced")
	}
	if !strings.Contains(string(entries["word/document.xml"]), `w:w="777"`) {
		t.Fatal("template section properties were not preserved")
	}
}

func zipEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	entries := make(map[string][]byte, len(zr.File))
	for _, file := range zr.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		entries[file.Name] = data
	}
	return entries
}
