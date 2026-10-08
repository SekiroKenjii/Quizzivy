import { describe, expect, it } from "vitest";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec } from "@tests/support/openapi";
import { storedGroupBundle } from "@tests/support/storedGroupBundle";

const spec = loadSpec();
const ajv = new Ajv({ strict: false });
addFormats(ajv);
const shape = (name: string) =>
  ajv.compile({ $ref: `#/components/schemas/${name}`, components: spec.components });
const input = shape("QuestionInput");
const stored = shape("StoredQuestionInput");
const storedGroup = shape("StoredQuestionGroupBundle");
const id = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const choice = (n: number) => ({
  type: "single_choice" as const,
  prompt: "Pick",
  points: 1,
  options: Array.from({ length: n }, (_, i) => ({
    text: String(i),
    isCorrect: i === 0,
  })),
});

describe("question metadata wire boundaries", () => {
  it("caps only submitted inputs and preserves strict nullable stored responses", () => {
    expect(input(choice(8))).toBe(true);
    expect(input(choice(9))).toBe(false);
    expect(stored({ ...choice(9), level: null, skill: null })).toBe(true);
    expect(stored(choice(9))).toBe(false);
    const bundle = {
      group: {
        id,
        title: "Group",
        members: [{ questionId: id, optionOrder: "fixed" as const }],
        stimuli: [],
        recordings: [],
      },
      questions: [{ id, input: choice(9) }],
    };
    expect(storedGroup(storedGroupBundle(bundle))).toBe(true);
  });
  it("accepts every closed enum value and rejects unknown or empty values", () => {
    for (const level of ["pre_a1", "a1", "a2", "b1", "b2", "c1", "c2"])
      expect(input({ ...choice(2), level })).toBe(true);
    for (const skill of [
      "grammar",
      "vocabulary",
      "reading",
      "listening",
      "writing",
      "speaking",
    ])
      expect(input({ ...choice(2), skill })).toBe(true);
    for (const field of ["level", "skill"])
      for (const value of ["", "unknown", 1])
        expect(input({ ...choice(2), [field]: value })).toBe(false);
    expect(input({ ...choice(2), level: null, skill: null })).toBe(true);
  });
  it("requires complete fixed facet keys and rejects absent draft skills", () => {
    const facets = shape("QuestionTypeFacets");
    const counts = {
      all: 0,
      single_choice: 0,
      multiple_choice: 0,
      true_false: 0,
      fill_blank: 0,
      short_answer: 0,
      levels: { pre_a1: 0, a1: 0, a2: 0, b1: 0, b2: 0, c1: 0, c2: 0 },
      skills: {
        grammar: 0,
        vocabulary: 0,
        reading: 0,
        listening: 0,
        writing: 0,
        speaking: 0,
      },
    };
    expect(facets(counts)).toBe(true);
    expect(facets({ ...counts, levels: { a1: 0 } })).toBe(false);
    const test = shape("Test");
    const body = {
      id,
      title: "Test",
      status: "draft",
      currentVersion: 0,
      totalPoints: 0,
      questionCount: 0,
      audioCount: 0,
      skills: [],
      assignments: { live: 0, scheduled: 0, closed: 0 },
      sections: [],
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    };
    expect(test(body)).toBe(true);
    expect(test({ ...body, skills: null })).toBe(false);
    const missing = Object.fromEntries(
      Object.entries(body).filter(([key]) => key !== "skills"),
    );
    expect(test(missing)).toBe(false);
  });
});
