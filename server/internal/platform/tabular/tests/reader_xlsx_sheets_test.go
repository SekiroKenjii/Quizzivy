package tabular_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/tabular"
)

func sheetElements(count int, relationshipID string) string {
	var out strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&out, `<sheet name="s%d" sheetId="%d" r:id="%s"/>`, i, i, relationshipID)
	}
	return out.String()
}

func relationshipElements(count int) string {
	var out strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&out, `<Relationship Id="x%d" Type="w" Target="e%d"/>`, i, i)
	}
	return out.String()
}

func withSheetList(t *testing.T, sheets, workbookRels, rootRels string) []byte {
	t.Helper()
	entries := workbookParts(nil)
	entries = replaceBody(entries, "xl/workbook.xml", `<sheet name="Users" sheetId="1" r:id="rId1"/>`, sheets)
	entries = replaceBody(entries, "xl/_rels/workbook.xml.rels", `</Relationships>`, workbookRels+`</Relationships>`)
	entries = replaceBody(entries, "_rels/.rels", `</Relationships>`, rootRels+`</Relationships>`)
	return archive(t, append(entries, sheetEntry(rosterRows(2)))...)
}

func TestTheSheetsAndRelationshipsTheLookupMultipliesAreBoundedBeforeExcelizeSeesThem(t *testing.T) {
	for name, tc := range map[string]struct {
		sheets, workbookRels, rootRels string
		want                           error
	}{
		"8,000 sheets over 8,000 relationships with one id": {
			sheetElements(8000, "1"), strings.Repeat(`<Relationship Id="1" Type="w" Target="worksheets/sheet1.xml"/>`, 8000), "", tabular.ErrTooManySheets,
		},
		"16,000 sheets": {sheetElements(16000, "rId1"), "", "", tabular.ErrTooManySheets},
		"257 sheets":    {sheetElements(257, "rId1"), "", "", tabular.ErrTooManySheets},
		"8,000 relationships with one id": {
			`<sheet name="Users" sheetId="1" r:id="rId1"/>`, strings.Repeat(`<Relationship Id="1" Type="w" Target="worksheets/sheet1.xml"/>`, 8000), "", tabular.ErrFormat,
		},
		"8,000 relationships with ids of their own": {
			`<sheet name="Users" sheetId="1" r:id="rId1"/>`, relationshipElements(8000), "", tabular.ErrFormat,
		},
		"1,001 relationships": {`<sheet name="Users" sheetId="1" r:id="rId1"/>`, relationshipElements(998), "", tabular.ErrFormat},
		"an id used twice": {
			`<sheet name="Users" sheetId="1" r:id="rId1"/>`, `<Relationship Id="rId1" Type="w" Target="other.xml"/>`, "", tabular.ErrFormat,
		},
		"an id used twice, the second after the usual ones": {
			`<sheet name="Users" sheetId="1" r:id="rId3"/>`, `<Relationship Id="rId3" Type="w" Target="other.xml"/>`, "", tabular.ErrFormat,
		},
		"16,000 relationships in the package's own": {
			`<sheet name="Users" sheetId="1" r:id="rId1"/>`, "",
			rootFiller(16000), tabular.ErrFormat,
		},
		"17 relationships in the package's own": {
			`<sheet name="Users" sheetId="1" r:id="rId1"/>`, "", rootFiller(16), tabular.ErrFormat,
		},
	} {
		t.Run(name, func(t *testing.T) {
			data := withSheetList(t, tc.sheets, tc.workbookRels, tc.rootRels)
			var err error
			start := time.Now()

			allocated := allocatedDuring(func() { _, _, err = read(t, data, tabular.FormatXLSX, tabular.DefaultLimits()) })

			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			if time.Since(start) > time.Second || allocated > 16<<20 {
				t.Errorf("refusing it took %v and allocated %d bytes, want the lookup never run", time.Since(start), allocated)
			}
		})
	}
}

func rootFiller(count int) string {
	var out strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&out, `<Relationship Id="f%d" Type="t" Target="d%d"/>`, i, i)
	}
	return out.String()
}

func TestAWorkbookAtEachBoundIsStillRead(t *testing.T) {
	for name, tc := range map[string]struct {
		sheets, workbookRels, rootRels string
	}{
		"256 sheets":                              {sheetElements(256, "rId1"), "", ""},
		"1,000 relationships":                     {`<sheet name="Users" sheetId="1" r:id="rId1"/>`, relationshipElements(997), ""},
		"16 relationships in the package's own":   {`<sheet name="Users" sheetId="1" r:id="rId1"/>`, "", rootFiller(15)},
		"a handful of sheets, each with its part": {`<sheet name="Users" sheetId="1" r:id="rId1"/><sheet name="Other" sheetId="2" r:id="rId9"/>`, `<Relationship Id="rId9" Type="w" Target="worksheets/sheet1.xml"/>`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, rows, err := read(t, withSheetList(t, tc.sheets, tc.workbookRels, tc.rootRels), tabular.FormatXLSX, tabular.DefaultLimits())

			if err != nil || len(rows) != 2 {
				t.Errorf("rows = %d, err = %v, want the 2 rows read", len(rows), err)
			}
		})
	}
}

func TestElementsNamedLikeRelationshipsInOtherPartsAreNotCounted(t *testing.T) {
	theme := entry{"xl/theme/theme1.xml", `<theme xmlns="http://schemas.openxmlformats.org/drawingml/2006/main">` + strings.Repeat(`<Relationship Id="a"/>`, 1500) + `</theme>`}

	_, rows, err := read(t, workbook(t, rosterRows(2), nil, theme), tabular.FormatXLSX, tabular.DefaultLimits())

	if err != nil || len(rows) != 2 {
		t.Errorf("rows = %d, err = %v, want the 2 rows read", len(rows), err)
	}
}

func TestTheSheetCountHasItsOwnMessage(t *testing.T) {
	got := fmt.Errorf("refused: %w", tabular.ErrTooManySheets)
	for _, other := range append([]error{tabular.ErrFormat, tabular.ErrLayout}, sizeSentinels...) {
		if errors.Is(got, other) || tabular.ErrTooManySheets.Error() == other.Error() {
			t.Errorf("ErrTooManySheets is %v", other)
		}
	}
	if !strings.Contains(got.Error(), "256") {
		t.Errorf("%q, want it to name the limit", got)
	}
}

func chartSheetFirst(t *testing.T, target string, part ...entry) []byte {
	t.Helper()
	entries := replaceBody(workbookParts(nil), "xl/workbook.xml",
		`<sheet name="Users" sheetId="1" r:id="rId1"/>`,
		`<sheet name="Chart" sheetId="2" r:id="rId9"/><sheet name="Users" sheetId="1" r:id="rId1"/>`)
	entries = replaceBody(entries, "xl/_rels/workbook.xml.rels", `</Relationships>`,
		`<Relationship Id="rId9" Type="`+relationsNS+`/chartsheet" Target="`+target+`"/></Relationships>`)
	return archive(t, append(append(entries, sheetEntry(rosterRows(2))), part...)...)
}

func TestAFirstSheetThatIsNotAWorksheetIsALayoutErrorNotAMissingHeader(t *testing.T) {
	chart := entry{"xl/chartsheets/sheet1.xml", `<chartsheet xmlns="` + spreadsheetNS + `"/>`}
	for name, data := range map[string][]byte{
		"a chart sheet":                   chartSheetFirst(t, "chartsheets/sheet1.xml", chart),
		"a part outside the scanned set":  chartSheetFirst(t, "externalLinks/e1.xml", entry{"xl/externalLinks/e1.xml", `<externalLink xmlns="` + spreadsheetNS + `"/>`}),
		"a part that is not there":        chartSheetFirst(t, "chartsheets/gone.xml"),
		"a rels part under worksheets":    chartSheetFirst(t, "worksheets/_rels/sheet1.xml.rels", entry{"xl/worksheets/_rels/sheet1.xml.rels", `<Relationships xmlns="` + packageRelsNS + `"/>`}),
		"an absolute target elsewhere":    chartSheetFirst(t, "/xl/chartsheets/sheet1.xml", chart),
		"a worksheet spelled in capitals": archive(t, append(workbookParts(nil), entry{"XL/WORKSHEETS/sheet1.xml", sheetEntry(rosterRows(2)).body})...),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := read(t, data, tabular.FormatXLSX, tabular.DefaultLimits())

			if !errors.Is(err, tabular.ErrLayout) || !errors.Is(err, tabular.ErrFormat) {
				t.Errorf("err = %v, want ErrLayout, which wraps ErrFormat", err)
			}
			if errors.Is(err, tabular.ErrEmpty) {
				t.Errorf("err = %v, must not tell the teacher the header is missing", err)
			}
		})
	}
}

func TestTheFirstSheetTheWorkbookNamesIsTheOneRead(t *testing.T) {
	chart := entry{"xl/chartsheets/sheet1.xml", `<chartsheet xmlns="` + spreadsheetNS + `"/>`}
	second := entry{"xl/worksheets/sheet2.xml", sheetEntry(rosterRows(3)).body}

	secondSheetIsTheChart := replaceBody(workbookParts(nil), "xl/workbook.xml",
		`<sheet name="Users" sheetId="1" r:id="rId1"/>`,
		`<sheet name="Users" sheetId="1" r:id="rId1"/><sheet name="Chart" sheetId="2" r:id="rId9"/>`)
	secondSheetIsTheChart = replaceBody(secondSheetIsTheChart, "xl/_rels/workbook.xml.rels", `</Relationships>`,
		`<Relationship Id="rId9" Type="`+relationsNS+`/chartsheet" Target="chartsheets/sheet1.xml"/></Relationships>`)

	otherPartFirst := replaceBody(workbookParts(nil), "xl/workbook.xml",
		`<sheet name="Users" sheetId="1" r:id="rId1"/>`,
		`<sheet name="Later" sheetId="2" r:id="rId8"/><sheet name="Users" sheetId="1" r:id="rId1"/>`)
	otherPartFirst = replaceBody(otherPartFirst, "xl/_rels/workbook.xml.rels", `</Relationships>`,
		`<Relationship Id="rId8" Type="`+relationsNS+`/worksheet" Target="worksheets/sheet2.xml"/></Relationships>`)

	for name, tc := range map[string]struct {
		data []byte
		rows int
	}{
		"a chart sheet after it":            {archive(t, append(append(secondSheetIsTheChart, sheetEntry(rosterRows(2))), chart)...), 2},
		"a worksheet that is not sheet1":    {archive(t, append(append(otherPartFirst, sheetEntry(rosterRows(2))), second)...), 3},
		"an absolute target":                {archive(t, append(replaceBody(workbookParts(nil), "xl/_rels/workbook.xml.rels", `Target="worksheets/sheet1.xml"`, `Target="/xl/worksheets/sheet1.xml"`), sheetEntry(rosterRows(2)))...), 2},
		"a target with a backslash":         {archive(t, append(replaceBody(workbookParts(nil), "xl/_rels/workbook.xml.rels", `Target="worksheets/sheet1.xml"`, `Target="worksheets\sheet1.xml"`), sheetEntry(rosterRows(2)))...), 2},
		"a target that climbs and returns":  {archive(t, append(replaceBody(workbookParts(nil), "xl/_rels/workbook.xml.rels", `Target="worksheets/sheet1.xml"`, `Target="../xl/worksheets/sheet1.xml"`), sheetEntry(rosterRows(2)))...), 2},
		"a workbook that does not say":      {archive(t, append(replaceBody(workbookParts(nil), "xl/workbook.xml", ` r:id="rId1"`, ``), sheetEntry(rosterRows(2)))...), -1},
		"a relationship that is not listed": {archive(t, append(replaceBody(workbookParts(nil), "xl/workbook.xml", `r:id="rId1"`, `r:id="rId7"`), sheetEntry(rosterRows(2)))...), -1},
	} {
		t.Run(name, func(t *testing.T) {
			_, rows, err := read(t, tc.data, tabular.FormatXLSX, tabular.DefaultLimits())

			switch {
			case tc.rows >= 0 && (err != nil || len(rows) != tc.rows):
				t.Errorf("rows = %d, err = %v, want the %d rows of the first sheet", len(rows), err, tc.rows)
			case tc.rows < 0 && errors.Is(err, tabular.ErrLayout):
				t.Errorf("err = %v, want the workbook's own answer when it does not say where its first sheet is", err)
			}
		})
	}
}
