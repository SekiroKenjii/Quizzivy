package tabular

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strings"

	"golang.org/x/net/html/charset"
)

// checkFirstSheet fails with ErrLayout when the first sheet of the workbook is
// not a worksheet that excelize will find under that exact name: a chart sheet
// or a part outside xl/worksheets/, and a worksheet whose folder is spelled in
// another case. It has no opinion, and returns nil, when the workbook does not
// say where its first sheet is, so excelize reports that as it always has. It
// runs after the parts were scanned, so they are small and well formed.
func checkFirstSheet(files []*zip.File) error {
	workbook, rels := findPart(files, workbookPart), findPart(files, workbookRelsPart)
	if workbook == nil || rels == nil {
		return nil
	}
	id, err := firstSheetID(workbook)
	if err != nil || id == "" {
		return nil
	}
	target, err := relationshipTarget(rels, id)
	if err != nil || target == "" {
		return nil
	}
	resolved := resolveTarget(target)
	if isSheetPart(partName(resolved)) && hasEntry(files, resolved) {
		return nil
	}
	return ErrLayout
}

func findPart(files []*zip.File, name string) *zip.File {
	for _, entry := range files {
		if partName(entry.Name) == name {
			return entry
		}
	}
	return nil
}

func hasEntry(files []*zip.File, name string) bool {
	for _, entry := range files {
		if strings.ReplaceAll(entry.Name, `\`, "/") == name {
			return true
		}
	}
	return false
}

func resolveTarget(target string) string {
	target = strings.ReplaceAll(target, `\`, "/")
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Join(path.Dir(workbookPart), target)
}

func firstSheetID(workbook *zip.File) (string, error) {
	var id string
	err := forEachStart(workbook, func(el xml.StartElement) bool {
		if el.Name.Local != "sheet" {
			return true
		}
		for _, attr := range el.Attr {
			if attr.Name.Local == "id" {
				id = attr.Value
			}
		}
		return false
	})
	return id, err
}

func relationshipTarget(rels *zip.File, id string) (string, error) {
	var target string
	err := forEachStart(rels, func(el xml.StartElement) bool {
		if el.Name.Local != "Relationship" {
			return true
		}
		var matches bool
		var found string
		for _, attr := range el.Attr {
			switch attr.Name.Local {
			case "Id":
				matches = attr.Value == id
			case "Target":
				found = attr.Value
			}
		}
		if matches {
			target = found
		}
		return !matches
	})
	return target, err
}

func forEachStart(entry *zip.File, visit func(xml.StartElement) bool) error {
	body, err := entry.Open()
	if err != nil {
		return err
	}
	dec := xml.NewDecoder(io.LimitReader(body, maxXMLPartBytes+1))
	dec.CharsetReader = charset.NewReaderLabel
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return errors.Join(err, body.Close())
		}
		if el, ok := tok.(xml.StartElement); ok && !visit(el) {
			return body.Close()
		}
	}
}
