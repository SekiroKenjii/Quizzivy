import { clipboardHTML } from "@/components/shared/content/editor/clipboardHTML";
import { CLIPBOARD_HTML_LIMIT } from "@/components/shared/content/editor/clipboardLimits";
import { clipboardStyles } from "@/components/shared/content/editor/clipboardStyles";
import { contentPlainText } from "@/components/shared/content/plainText";

test("preserves Vietnamese text, combined semantic marks, safe links and Word paragraphs", () => {
  const result = clipboardHTML(`<html><head><meta charset="utf-8"></head><body>
    <!--StartFragment--><p class="MsoNormal" style="font-family:Arial;margin:0;color:red">Chọn <strong><span style="text-decoration:underline;font-style:italic">từ đúng</span></strong><br>H<sub>2</sub>O và x<sup>2</sup>.</p>
    <p><a href="https://example.test/đề">Tham khảo</a></p><!--EndFragment-->
    </body></html>`);
  expect(result).not.toBeNull();
  expect(contentPlainText(result!)).toBe("Chọn từ đúng\nH2O và x2.\n\nTham khảo");
  expect(JSON.stringify(result)).toContain('"marks":["bold","underline","italic"]');
  expect(JSON.stringify(result)).not.toMatch(/MsoNormal|Arial|red|style/);
  expect(JSON.stringify(result)).toContain('"href":"https://example.test/đề"');
});

test("preserves list starts, table cells, spans and nested lists without inferring answers", () => {
  const result = clipboardHTML(
    '<ol start="4"><li><p><u>Đáp án A</u></p><ul><li>ghi chú</li></ul></li></ol><table><tr><th colspan="2">Buổi học</th></tr><tr><td rowspan="2">Sáng</td><td>Thứ hai</td></tr><tr><td>Thứ ba</td></tr></table>',
  );
  expect(result?.blocks[0]).toMatchObject({ type: "list", ordered: true, start: 4 });
  expect(result?.blocks[1]).toMatchObject({
    type: "table",
    rows: [
      [{ header: true, rowSpan: 1, colSpan: 2 }],
      [
        { rowSpan: 2, colSpan: 1 },
        { rowSpan: 1, colSpan: 1 },
      ],
      [{ rowSpan: 1, colSpan: 1 }],
    ],
  });
  expect(contentPlainText(result!)).toContain("Buổi học\nSáng\tThứ hai\nThứ ba");
  expect(JSON.stringify(result)).not.toMatch(/isCorrect|acceptedAnswers|gap/);
});

test("CSS overrides retain independent marks and reject ambiguous or hidden meaning", () => {
  expect(
    clipboardStyles("font-weight:normal;font-style:italic", ["bold", "underline"]),
  ).toEqual(["underline", "italic"]);
  expect(
    clipboardStyles("font-weight:700!important;vertical-align:sub", ["superscript"]),
  ).toEqual(["bold", "subscript"]);
  expect(() => clipboardStyles("display:none", [])).toThrow();
  expect(() => clipboardStyles("text-transform:uppercase", [])).toThrow();
  expect(clipboardStyles("text-decoration:underline;text-decoration:none", [])).toEqual(
    [],
  );
  expect(clipboardStyles("text-decoration:none", [], ["underline"])).toEqual([]);
  expect(clipboardStyles("text-decoration:none", ["underline"])).toEqual(["underline"]);
});

test.each([
  '<p>A<img src="https://example.test/tracker">B</p>',
  '<p>A<script>fetch("https://example.test")</script>B</p>',
  '<p>A<iframe src="https://example.test"></iframe>B</p>',
  '<p onclick="alert(1)">A</p>',
  '<p style="background:url(https://example.test)">A</p>',
  '<p style="background:u\\72l(https://example.test)">A</p>',
  '<p><a href="javascript:alert(1)">A</a></p>',
  '<p><a href="http://example.test">A</a></p>',
  '<p><a href="https://example.test"><br>A</a></p>',
  '<p><span data-gap-id="injected">1</span></p>',
  "<p>A<del>B</del>C</p>",
  "<h4>A</h4>",
  '<p style="mso-list:l0 level1 lfo1">A</p>',
  "<ol reversed><li>A</li></ol>",
  '<ol><li value="3">A</li></ol>',
  "<p>Before</p><tr><td>discarded tags</td></tr>",
  '<p class="one" class="two">A</p>',
  "<table><tr><td>A</td><td>B</td></tr><tr><td>C</td></tr></table>",
  "<table><tr><td><table><tr><td>A</td></tr></table></td></tr></table>",
  '<table><tr><td rowspan="0">A</td></tr></table>',
  '<p style="white-space:pre"> A  B </p>',
  "<p><svg><text>A</text></svg></p>",
  '<style>.answer {text-decoration:underline}</style><p class="answer">A</p>',
  "<p>A\u0000B</p>",
  "<p>A\ud800B</p>",
])("refuses unsupported source without a partially stripped candidate: %s", (html) => {
  expect(clipboardHTML(html)).toBeNull();
});

test("bounds input bytes, tree depth and output text before a candidate can be used", () => {
  expect(clipboardHTML("a".repeat(CLIPBOARD_HTML_LIMIT + 1))).toBeNull();
  expect(clipboardHTML("ệ".repeat(CLIPBOARD_HTML_LIMIT / 2))).toBeNull();
  expect(clipboardHTML("<div>".repeat(50) + "x" + "</div>".repeat(50))).toBeNull();
  expect(clipboardHTML("<p>x</p>".repeat(3100))).toBeNull();
  expect(clipboardHTML("<p>" + "a".repeat(100_001) + "</p>")).toBeNull();
});

test("collapses HTML layout whitespace without collapsing nonbreaking spaces or accents", () => {
  const result = clipboardHTML(
    "<div>\n <p>A <b> B </b> C&nbsp;D e\u0302</p>\n<p></p></div>",
  );
  expect(contentPlainText(result!)).toBe("A B C\u00a0D e\u0302\n\n");
});
