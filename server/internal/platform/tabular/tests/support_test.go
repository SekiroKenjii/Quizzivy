package tabular_test

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"fmt"
	"hash/crc32"
	"runtime"
	"strings"
	"testing"

	"quizzivy/internal/platform/tabular"
)

const (
	spreadsheetNS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	relationsNS   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	packageRelsNS = "http://schemas.openxmlformats.org/package/2006/relationships"
	overrideType  = "application/vnd.openxmlformats-officedocument.spreadsheetml."
)

type entry struct {
	name string
	body string
}

func read(t *testing.T, data []byte, format tabular.Format, limits tabular.Limits) ([]string, []tabular.Row, error) {
	t.Helper()
	table, err := tabular.ReadTable(bytes.NewReader(data), int64(len(data)), format, limits)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := table.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	var rows []tabular.Row
	for row, err := range table.Rows() {
		if err != nil {
			return table.Header, rows, err
		}
		rows = append(rows, row)
	}
	return table.Header, rows, nil
}

func archive(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func workbook(t *testing.T, sheetData string, shared []string, extra ...entry) []byte {
	t.Helper()
	var strs strings.Builder
	for _, s := range shared {
		strs.WriteString("<si><t>" + s + "</t></si>")
	}
	entries := []entry{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="` + overrideType + `sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="` + overrideType + `worksheet+xml"/>` +
			`<Override PartName="/xl/sharedStrings.xml" ContentType="` + overrideType + `sharedStrings+xml"/>` +
			`<Override PartName="/xl/styles.xml" ContentType="` + overrideType + `styles+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="` + packageRelsNS + `"><Relationship Id="rId1" ` +
			`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="` + spreadsheetNS + `" xmlns:r="` + relationsNS + `">` +
			`<sheets><sheet name="Users" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="` + packageRelsNS + `">` +
			`<Relationship Id="rId1" Type="` + relationsNS + `/worksheet" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="` + relationsNS + `/sharedStrings" Target="sharedStrings.xml"/>` +
			`<Relationship Id="rId3" Type="` + relationsNS + `/styles" Target="styles.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<worksheet xmlns="` + spreadsheetNS + `"><sheetData>` + sheetData + `</sheetData></worksheet>`},
		{"xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<sst xmlns="` + spreadsheetNS + `" count="` + fmt.Sprint(len(shared)) + `" uniqueCount="` + fmt.Sprint(len(shared)) + `">` + strs.String() + `</sst>`},
		{"xl/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<styleSheet xmlns="` + spreadsheetNS + `"><numFmts count="1"><numFmt numFmtId="164" formatCode="yyyy\-mm\-dd"/></numFmts>` +
			`<fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>` +
			`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
			`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
			`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
			`<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
			`<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/></cellXfs></styleSheet>`},
	}
	return archive(t, append(entries, extra...)...)
}

func text(ref, value string) string {
	return `<c r="` + ref + `" t="inlineStr"><is><t>` + value + `</t></is></c>`
}

func sharedString(ref string, index int) string {
	return fmt.Sprintf(`<c r="%s" t="s"><v>%d</v></c>`, ref, index)
}

func row(number int, cells ...string) string {
	return fmt.Sprintf(`<row r="%d">%s</row>`, number, strings.Join(cells, ""))
}

func archiveOfBombs(t *testing.T, names []string, size int) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	chunk := make([]byte, 1<<20)
	for _, name := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		for written := 0; written < size; written += len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func archiveDeclaring(t *testing.T, name string, declared uint64, flags uint16) []byte {
	t.Helper()
	var packed bytes.Buffer
	fw, err := flate.NewWriter(&packed, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("tiny")
	if _, err := fw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name:               name,
		Method:             zip.Deflate,
		Flags:              flags,
		CRC32:              crc32.ChecksumIEEE(payload),
		CompressedSize64:   uint64(packed.Len()),
		UncompressedSize64: declared,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(packed.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func allocatedDuring(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}
