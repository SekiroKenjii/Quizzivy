package tabular

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"golang.org/x/net/html/charset"
)

const (
	relationshipsNS       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	strictRelationshipsNS = "http://purl.oclc.org/ooxml/officeDocument/relationships"
)

func checkFirstSheet(files []*zip.File) error {
	workbook, rels := findPart(files, workbookPart), findPart(files, workbookRelsPart)
	if workbook == nil || rels == nil {
		return nil
	}
	id, err := firstSheetID(workbook)
	if err != nil || id == "" {
		return nil
	}
	target, kind, err := relationshipTarget(rels, id)
	if err != nil || target == "" {
		return nil
	}
	if kind == "chartsheet" || kind == "dialogsheet" {
		return ErrNotWorksheet
	}
	resolved := resolveTarget(target)
	switch {
	case hasEntry(files, resolved) && isSheetPart(partName(resolved)):
		return nil
	case hasEntry(files, resolved) || hasEntryFold(files, resolved):
		return ErrLayout
	}
	return fmt.Errorf("%w: the first sheet's part is not in the file", ErrFormat)
}

func findPart(files []*zip.File, name string) *zip.File {
	var found *zip.File
	for _, entry := range files {
		if strings.ReplaceAll(entry.Name, `\`, "/") == name {
			found = entry
		}
	}
	return found
}

func hasEntry(files []*zip.File, name string) bool {
	return findPart(files, name) != nil
}

func hasEntryFold(files []*zip.File, name string) bool {
	return slices.ContainsFunc(files, func(entry *zip.File) bool {
		return strings.EqualFold(strings.ReplaceAll(entry.Name, `\`, "/"), name)
	})
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
			if attr.Name.Local == "id" && (attr.Name.Space == relationshipsNS || attr.Name.Space == strictRelationshipsNS) {
				id = attr.Value
			}
		}
		return false
	})
	return id, err
}

func relationshipTarget(rels *zip.File, id string) (target, kind string, err error) {
	err = forEachStart(rels, func(el xml.StartElement) bool {
		if el.Name.Local != "Relationship" {
			return true
		}
		var matches bool
		var foundTarget, foundType string
		for _, attr := range el.Attr {
			switch attr.Name.Local {
			case "Id":
				matches = attr.Value == id
			case "Target":
				foundTarget = attr.Value
			case "Type":
				foundType = attr.Value
			}
		}
		if matches {
			target, kind = foundTarget, path.Base(foundType)
		}
		return !matches
	})
	return target, kind, err
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
