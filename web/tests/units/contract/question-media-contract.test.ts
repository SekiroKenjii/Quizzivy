import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec, SPEC_PATH } from "@tests/support/openapi";
import { QUESTION_MEDIA_LIMITS } from "@/components/shared/content/model";
import { contentPlainText } from "@/components/shared/content/plainText";
import {
  isQuestionContent,
  placesAudio,
  questionMedia,
  validateQuestionProse,
} from "@/components/shared/content/questionContent";

/**
 * api/testdata/question-media-v1.json is the server's statement of DG-109's
 * media rules (D-064). The schema must accept every document the server
 * accepts, and this client's guards must take and refuse exactly what the
 * server does, projecting each document to the server's plain text.
 */
type Case = {
  name: string;
  profile: "prompt" | "explanation";
  document: unknown;
  valid: boolean;
  plainText?: string;
  assets?: { id: string; kind: "image" | "audio" }[];
  placesAudio?: boolean;
};

const cases = JSON.parse(
  readFileSync(resolve(SPEC_PATH, "../testdata/question-media-v1.json"), "utf8"),
) as Case[];
const doc = loadSpec();
const ajv = new Ajv({ strict: false, validateFormats: true });
addFormats(ajv);
const schemas = {
  prompt: ajv.compile({
    $ref: "#/components/schemas/QuestionPromptContent",
    components: doc.components,
  }),
  explanation: ajv.compile({
    $ref: "#/components/schemas/QuestionExplanationContent",
    components: doc.components,
  }),
};

test.each(cases.filter((fixture) => fixture.valid))(
  "$name is a document the contract describes",
  (fixture) => {
    const shape = schemas[fixture.profile];
    expect(shape(fixture.document), JSON.stringify(shape.errors)).toBe(true);
  },
);

test.each(cases.filter((fixture) => fixture.valid))(
  "$name is taken by this client as the server takes it",
  (fixture) => {
    const parsed = validateQuestionProse(fixture.document, fixture.profile);
    expect(parsed.ok).toBe(true);
    if (!parsed.ok) return;
    expect(contentPlainText(parsed.value)).toBe(fixture.plainText);
    expect(questionMedia(parsed.value)).toEqual(fixture.assets);
    expect(placesAudio(parsed.value)).toBe(fixture.placesAudio ?? false);
    expect(isQuestionContent(fixture.document)).toBe(false);
  },
);

test.each(cases.filter((fixture) => !fixture.valid))(
  "$name is refused by this client too",
  (fixture) => {
    expect(validateQuestionProse(fixture.document, fixture.profile).ok).toBe(false);
  },
);

test("the client's media limit is the contract's x-media-blocks", () => {
  const schemas = doc.components.schemas as Record<string, Record<string, unknown>>;
  expect(schemas["QuestionPromptContent"]?.["x-media-blocks"]).toBe(
    QUESTION_MEDIA_LIMITS.blocks,
  );
  expect(schemas["QuestionExplanationContent"]?.["x-media-blocks"]).toBe(
    QUESTION_MEDIA_LIMITS.blocks,
  );
});

test("a media block inside a list item is no document the contract describes", () => {
  const nested = cases.find(
    (fixture) => fixture.name === "an image in a list item is refused",
  )!;
  expect(schemas.prompt(nested.document)).toBe(false);
});
