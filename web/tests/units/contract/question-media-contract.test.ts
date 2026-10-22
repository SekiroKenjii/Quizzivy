import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec, SPEC_PATH } from "@tests/support/openapi";
import {
  isQuestionContent,
  isQuestionPromptContent,
} from "@/components/shared/content/questionContent";

/**
 * api/testdata/question-media-v1.json is the server's statement of DG-109's
 * media rules (D-064). The schema must accept every document the server
 * accepts, and until R6-14 draws media this client's guards refuse each one,
 * as the bank write does until R6-13.
 */
type Case = {
  name: string;
  profile: "prompt" | "explanation";
  document: unknown;
  valid: boolean;
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
  "$name is refused by this client until R6-14",
  (fixture) => {
    expect(isQuestionPromptContent(fixture.document)).toBe(false);
    expect(isQuestionContent(fixture.document)).toBe(false);
  },
);

test("a media block inside a list item is no document the contract describes", () => {
  const nested = cases.find(
    (fixture) => fixture.name === "an image in a list item is refused",
  )!;
  expect(schemas.prompt(nested.document)).toBe(false);
});
