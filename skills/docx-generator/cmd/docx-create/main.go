package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

type input struct {
	Title      string      `json:"title"`
	Paragraphs []paragraph `json:"paragraphs"`
}

type paragraph struct {
	Text  string `json:"text"`
	Style string `json:"style"`
}

func main() {
	inputPath := flag.String("input", "", "путь к JSON с содержимым")
	outputPath := flag.String("output", "", "путь к создаваемому DOCX")
	templatePath := flag.String("template", "", "необязательный DOCX-образец стилей")
	flag.Parse()
	if err := run(*inputPath, *outputPath, *templatePath); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(inputPath, outputPath, templatePath string) error {
	if inputPath == "" || outputPath == "" {
		return errors.New("--input and --output are required")
	}
	if filepath.Clean(inputPath) == filepath.Clean(outputPath) || (templatePath != "" && filepath.Clean(templatePath) == filepath.Clean(outputPath)) {
		return errors.New("output must differ from input and template")
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
	var content input
	if err := json.Unmarshal(raw, &content); err != nil {
		return fmt.Errorf("input JSON: %w", err)
	}
	entries := baseEntries()
	section := ""
	if templatePath != "" {
		entries, section, err = readTemplate(templatePath)
		if err != nil {
			return err
		}
	}
	entries["word/document.xml"] = documentXML(content, section)
	return writeDocx(outputPath, entries)
}

func readTemplate(path string) (map[string][]byte, string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, "", fmt.Errorf("open template: %w", err)
	}
	defer zr.Close()
	entries := make(map[string][]byte, len(zr.File))
	section := ""
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return nil, "", err
		}
		data, readErr := io.ReadAll(r)
		r.Close()
		if readErr != nil {
			return nil, "", readErr
		}
		if file.Name == "word/document.xml" {
			section = extractSection(string(data))
			continue
		}
		entries[file.Name] = data
	}
	if _, ok := entries["[Content_Types].xml"]; !ok {
		return nil, "", errors.New("template is not a DOCX package")
	}
	return entries, section, nil
}

func writeDocx(path string, entries map[string][]byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			f.Close()
			return err
		}
		if _, err := w.Write(entries[name]); err != nil {
			zw.Close()
			f.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func documentXML(content input, section string) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	if content.Title != "" {
		writeParagraph(&b, content.Title, "Title")
	}
	for _, p := range content.Paragraphs {
		writeParagraph(&b, p.Text, p.Style)
	}
	if section == "" {
		section = `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/></w:sectPr>`
	}
	b.WriteString(section)
	b.WriteString(`</w:body></w:document>`)
	return b.Bytes()
}

var sectionPattern = regexp.MustCompile(`(?s)<w:sectPr(?:\s[^>]*)?>.*?</w:sectPr>`)

func extractSection(document string) string {
	return sectionPattern.FindString(document)
}

func writeParagraph(b *bytes.Buffer, text, style string) {
	b.WriteString("<w:p>")
	if style != "" {
		b.WriteString(`<w:pPr><w:pStyle w:val="`)
		writeEscaped(b, style)
		b.WriteString(`"/></w:pPr>`)
	}
	b.WriteString(`<w:r><w:t xml:space="preserve">`)
	writeEscaped(b, text)
	b.WriteString("</w:t></w:r></w:p>")
}

func writeEscaped(b *bytes.Buffer, value string) {
	_ = xml.EscapeText(b, []byte(value))
}

func baseEntries() map[string][]byte {
	return map[string][]byte{
		"[Content_Types].xml":          []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`),
		"_rels/.rels":                  []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`),
		"word/_rels/document.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`),
		"word/styles.xml":              []byte(wordStyles),
	}
}

const wordStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Aptos" w:hAnsi="Aptos" w:cs="Aptos"/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style><w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="40"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="32"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="26"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:rPr><w:b/><w:sz w:val="24"/></w:rPr></w:style></w:styles>`
