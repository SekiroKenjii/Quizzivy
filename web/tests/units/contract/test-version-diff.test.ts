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
  kind: "version",
  version: 1,
  publishedAt: "2026-01-01T00:00:00Z",
};
const test = {
  id,
  title: "Test",
  status: "published",
  currentVersion: 1,
  totalPoints: 10,
  questionCount: 2,
  audioCount: 0,
  skills: [],
  assignments: { live: 0, scheduled: 0, closed: 0 },
  unpublishedChanges: 0,
  sections: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};
const without = (body: Record<string, unknown>, key: string) =>
  Object.fromEntries(Object.entries(body).filter(([name]) => name !== key));

describe("test version diff wire boundaries", () => {
  it("requires unpublishedChanges on a test, as a non-negative integer or null", () => {
    const body = shape("Test");
    expect(body(test)).toBe(true);
    expect(body({ ...test, unpublishedChanges: 12 })).toBe(true);
    expect(body({ ...test, unpublishedChanges: null })).toBe(true);
    expect(body({ ...test, unpublishedChanges: -1 })).toBe(false);
    expect(body({ ...test, unpublishedChanges: 1.5 })).toBe(false);
    expect(body({ ...test, unpublishedChanges: "3" })).toBe(false);
    expect(body(without(test, "unpublishedChanges"))).toBe(false);
  });

  it("describes a side as the draft or a version with the time it was published", () => {
    const side = shape("DiffSide");
    expect(side({ kind: "draft" })).toBe(true);
    expect(side(version)).toBe(true);
    expect(side({ kind: "latest" })).toBe(false);
    expect(side({ kind: "version", version: 0 })).toBe(false);
    expect(side({ version: 1 })).toBe(false);
  });

  it("names the five kinds of change and nothing else", () => {
    const kind = shape("DiffChangeKind");
    for (const name of ["added", "removed", "changed", "answer", "points"]) {
      expect(kind(name)).toBe(true);
    }
    expect(kind("moved")).toBe(false);
    expect(kind("reordered")).toBe(false);
  });

  it("carries each kind's params in one closed object", () => {
    const change = shape("DiffChange");
    expect(
      change({
        kind: "added",
        questionNumber: 5,
        questionId: id,
        params: { prompt: "Câu mới" },
      }),
    ).toBe(true);
    expect(
      change({
        kind: "changed",
        questionNumber: 4,
        questionId: id,
        params: { fields: ["prompt", "context"] },
      }),
    ).toBe(true);
    expect(
      change({
        kind: "answer",
        questionNumber: 2,
        questionId: id,
        params: { answerFrom: ["B"], answerTo: ["A"] },
      }),
    ).toBe(true);
    expect(change({ kind: "points", params: { pointsFrom: 10, pointsTo: 10.5 } })).toBe(
      true,
    );
    expect(change({ kind: "removed", params: {} })).toBe(true);

    expect(change({ kind: "changed", params: { fields: [] } })).toBe(false);
    expect(change({ kind: "changed", params: { fields: ["prompt", "prompt"] } })).toBe(
      false,
    );
    expect(change({ kind: "changed", params: { fields: ["transcript"] } })).toBe(false);
    expect(change({ kind: "added", params: { prompt: "x".repeat(201) } })).toBe(false);
    expect(change({ kind: "added", params: { title: "Câu" } })).toBe(false);
    expect(change({ kind: "added", questionNumber: 0, params: {} })).toBe(false);
    expect(change({ kind: "added", questionId: "not-a-uuid", params: {} })).toBe(false);
    expect(change({ kind: "added" })).toBe(false);
  });

  it("answers the first version's from side as null, not as a missing key", () => {
    const diff = shape("TestVersionDiff");
    const to = version;
    expect(diff({ from: null, to, changes: [] })).toBe(true);
    expect(
      diff({
        from: { kind: "version", version: 1, publishedAt: "2026-01-01T00:00:00Z" },
        to: { kind: "draft" },
        changes: [],
      }),
    ).toBe(true);
    expect(diff({ to, changes: [] })).toBe(false);
    expect(diff({ from: null, to, changes: null })).toBe(false);
    expect(diff({ from: null, changes: [] })).toBe(false);
  });

  it("requires against, which is previous, draft or a version number, on a read-only GET", () => {
    const operation = spec.paths["/teacher/tests/{id}/versions/{version}/diff"].get;
    expect(operation.operationId).toBe("getTestVersionDiff");
    expect(operation["x-permission"]).toBe("workspace.teacher");
    const against = operation.parameters.find(
      (parameter: { name: string }) => parameter.name === "against",
    );
    expect(against.in).toBe("query");
    expect(against.required).toBe(true);
    const accepts = ajv.compile(against.schema);
    for (const value of ["previous", "draft", "1", "42", "100"]) {
      expect(accepts(value)).toBe(true);
    }
    for (const value of [
      "",
      "0",
      "01",
      "-1",
      "latest",
      "previous ",
      "draft,previous",
      "1.5",
    ]) {
      expect(accepts(value)).toBe(false);
    }
    expect(Object.keys(operation.responses).sort()).toEqual([
      "200",
      "400",
      "404",
      "422",
    ]);
  });
});
