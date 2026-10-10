import { describe, expect, it } from "vitest";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec } from "@tests/support/openapi";

const spec = loadSpec();
const ajv = new Ajv({ strict: false });
addFormats(ajv);
const test = ajv.compile({ $ref: "#/components/schemas/Test", components: spec.components });

const published = {
  id: "019535d9-3df7-79fb-b466-fa907fa17f9e",
  title: "Test",
  status: "published",
  currentVersion: 1,
  nextVersion: 4,
  totalPoints: 10,
  questionCount: 2,
  audioCount: 0,
  skills: [],
  assignments: { live: 0, scheduled: 0, closed: 0 },
  unpublishedChanges: null,
  sections: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

function without(body: Record<string, unknown>, key: string) {
  return Object.fromEntries(Object.entries(body).filter(([name]) => name !== key));
}

describe("Test.nextVersion on the wire", () => {
  it("is a required positive integer", () => {
    expect(test(published)).toBe(true);
    expect(test(without(published, "nextVersion"))).toBe(false);
    expect(test({ ...published, nextVersion: 0 })).toBe(false);
    expect(test({ ...published, nextVersion: 2.5 })).toBe(false);
    expect(test({ ...published, nextVersion: "4" })).toBe(false);
  });
});
