import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec, propertyNames, SPEC_PATH } from "@tests/support/openapi";
import { CONTENT_LIMITS } from "@/components/shared/content/model";
import { validateContent } from "@/components/shared/content/validation";
import { contentPlainText } from "@/components/shared/content/plainText";

type Case = {
  name: string;
  document: unknown;
  valid: boolean;
  schemaValid: boolean;
  plainText?: string;
};

const cases = JSON.parse(
  readFileSync(resolve(SPEC_PATH, "../testdata/content-v1.json"), "utf8"),
) as Case[];
const doc = loadSpec();
const ajv = new Ajv({ strict: false, validateFormats: true });
addFormats(ajv);
const shape = ajv.compile({
  $ref: "#/components/schemas/ContentDocument",
  components: doc.components,
});

test.each(cases)(
  "$name agrees with domain validation and schema constraints",
  (fixture) => {
    expect(shape(fixture.document), JSON.stringify(shape.errors)).toBe(
      fixture.schemaValid,
    );
    const parsed = validateContent(fixture.document);
    expect(parsed.ok).toBe(fixture.valid);
    if (parsed.ok) expect(contentPlainText(parsed.value)).toBe(fixture.plainText);
  },
);

test("content budget constants match the contract", () => {
  expect(CONTENT_LIMITS).toEqual(
    doc.components.schemas.ContentDocument["x-content-limits"],
  );
});

test("content vocabulary carries no answer keys or provenance", () => {
  const names = propertyNames(doc, doc.components.schemas.ContentDocument);
  for (const field of [
    "isCorrect",
    "acceptedAnswers",
    "sampleAnswer",
    "transcript",
    "transcripts",
    "sourcePath",
    "teacherNote",
  ])
    expect(names.has(field)).toBe(false);
});

test("counts Unicode scalars and rejects aggregate, depth and node overflow", () => {
  expect(
    validateContent({
      format: "legacy_markdown_v1",
      markdown: "🌱".repeat(CONTENT_LIMITS.text),
    }).ok,
  ).toBe(true);
  expect(
    validateContent({
      format: "legacy_markdown_v1",
      markdown: "🌱".repeat(CONTENT_LIMITS.text + 1),
    }).ok,
  ).toBe(false);
  const paragraphs = Array.from({ length: 1025 }, () => ({
    type: "paragraph",
    content: [{ type: "text", text: "x", marks: [] }],
  }));
  expect(
    validateContent({ format: "semantic_v1", blocks: paragraphs.slice(0, 1024) }).ok,
  ).toBe(true);
  expect(validateContent({ format: "semantic_v1", blocks: paragraphs }).ok).toBe(false);
  const textNode = {
    type: "paragraph",
    content: [{ type: "text", text: "x".repeat(50001), marks: [] }],
  };
  expect(
    validateContent({ format: "semantic_v1", blocks: [textNode, textNode] }).ok,
  ).toBe(false);
  let deep: unknown = { type: "paragraph", content: [] };
  for (let i = 0; i < CONTENT_LIMITS.depth; i++)
    deep = {
      type: "list",
      ordered: false,
      start: 1,
      items: [[{ type: "paragraph", content: [] }, deep]],
    };
  expect(validateContent({ format: "semantic_v1", blocks: [deep] }).ok).toBe(false);
});
