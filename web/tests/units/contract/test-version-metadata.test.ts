import { describe, expect, it } from "vitest";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec } from "@tests/support/openapi";

const spec = loadSpec();
const ajv = new Ajv({ strict: false });
addFormats(ajv);
const shape = (name: string) =>
  ajv.compile({ $ref: `#/components/schemas/${name}`, components: spec.components });
const id = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const version = {
  id,
  version: 1,
  totalPoints: 10,
  questionCount: 2,
  audioCount: 0,
  manualCount: 0,
  skills: [],
  publishedAt: "2026-01-01T00:00:00Z",
  publishedBy: "Thuong",
  assignmentCount: 0,
  changeNote: null,
};
const without = (body: Record<string, unknown>, key: string) =>
  Object.fromEntries(Object.entries(body).filter(([name]) => name !== key));

describe("test version metadata wire boundaries", () => {
  it("requires assignmentCount and answers changeNote as text of at most 200 characters or null", () => {
    const testVersion = shape("TestVersion");
    expect(testVersion(version)).toBe(true);
    expect(testVersion({ ...version, assignmentCount: 3 })).toBe(true);
    expect(testVersion({ ...version, changeNote: "Sửa câu 2" })).toBe(true);
    expect(testVersion({ ...version, changeNote: "đ".repeat(200) })).toBe(true);
    expect(testVersion({ ...version, changeNote: "đ".repeat(201) })).toBe(false);
    expect(testVersion(without(version, "assignmentCount"))).toBe(false);
    expect(testVersion(without(version, "changeNote"))).toBe(false);
    expect(testVersion({ ...version, assignmentCount: -1 })).toBe(false);
    expect(testVersion({ ...version, assignmentCount: null })).toBe(false);
  });
  it("requires skills, each one of the six question skills", () => {
    const testVersion = shape("TestVersion");
    expect(testVersion({ ...version, skills: ["grammar", "reading"] })).toBe(true);
    expect(
      testVersion({
        ...version,
        skills: [
          "grammar",
          "vocabulary",
          "reading",
          "listening",
          "writing",
          "speaking",
        ],
      }),
    ).toBe(true);
    expect(testVersion(without(version, "skills"))).toBe(false);
    expect(testVersion({ ...version, skills: null })).toBe(false);
    expect(testVersion({ ...version, skills: ["maths"] })).toBe(false);
    expect(testVersion({ ...version, skills: [null] })).toBe(false);
  });
  it("takes testUpdatedAt as an optional timestamp, which only the publish answer carries", () => {
    const testVersion = shape("TestVersion");
    expect(
      testVersion({ ...version, testUpdatedAt: "2026-01-01T00:00:00.123456Z" }),
    ).toBe(true);
    expect(testVersion({ ...version, testUpdatedAt: "yesterday" })).toBe(false);
    expect(testVersion({ ...version, testUpdatedAt: null })).toBe(false);
  });
  it("requires the three assignment counts of a test, each a non-negative integer", () => {
    const counts = shape("TestAssignmentCounts");
    expect(counts({ live: 2, scheduled: 0, closed: 5 })).toBe(true);
    expect(counts({ live: 2, scheduled: 0 })).toBe(false);
    expect(counts({ live: -1, scheduled: 0, closed: 0 })).toBe(false);
    expect(counts({ live: 1.5, scheduled: 0, closed: 0 })).toBe(false);
    expect(counts({ live: 0, scheduled: 0, closed: 0, draft: 0 })).toBe(false);

    const test = shape("Test");
    const body = {
      id,
      title: "Test",
      status: "draft",
      currentVersion: 0,
      nextVersion: 1,
      totalPoints: 0,
      questionCount: 0,
      audioCount: 0,
      skills: [],
      assignments: { live: 0, scheduled: 0, closed: 0 },
      unpublishedChanges: null,
      sections: [],
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    };
    expect(test(body)).toBe(true);
    expect(test(without(body, "assignments"))).toBe(false);
  });
  it("lets publishTest take no body, an empty one, or a note of at most 200 characters", () => {
    const operation = spec.paths["/teacher/tests/{id}/publish"].post;
    expect(operation.requestBody.required).not.toBe(true);
    const body = ajv.compile(operation.requestBody.content["application/json"].schema);
    expect(body({})).toBe(true);
    expect(body({ changeNote: null })).toBe(true);
    expect(body({ changeNote: "Thêm phần Nghe" })).toBe(true);
    expect(body({ changeNote: "đ".repeat(200) })).toBe(true);
    expect(body({ changeNote: "đ".repeat(201) })).toBe(false);
    expect(body({ changeNote: 12 })).toBe(false);
    expect(body({ changeNote: "x", publishedBy: "someone" })).toBe(false);
    expect(Object.keys(operation.responses)).toContain("400");
  });
});
