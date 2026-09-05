import { describe, expect, it } from "vitest";
import {
  collectRefs,
  isPublic,
  jsonResponseSchema,
  loadSpec,
  operations,
  propertyNames,
  resolveRef,
  type Json,
} from "@tests/support/openapi";
import { MAX_BYTES, MAX_DURATION_MS } from "@/features/media/limits";

/**
 * Structural invariants of api/openapi.yaml. Ported from api/contract_check.py,
 * which existed only because the web project did not yet — that file is now
 * deleted (T-0.7 said it would be).
 */

const doc = loadSpec();
const ops = operations(doc);

/** §13.5. `transcript` is the one exception, on the result endpoint only. */
const FORBIDDEN = [
  "isCorrect",
  "sampleAnswer",
  "acceptedAnswers",
  "transcript",
  "teacherNote",
] as const;
const TRANSCRIPT_ALLOWED_AT = new Set(["/app/attempts/{id}/result"]);

describe("references", () => {
  it("has no dangling $ref", () => {
    const dangling = [...new Set(collectRefs(doc))]
      .filter((r) => r.startsWith("#/"))
      .filter((r) => resolveRef(doc, r) === undefined);
    expect(dangling).toEqual([]);
  });
});

describe("§13.5: the student-payload boundary", () => {
  it("exposes no grading key from any /app/* success response", () => {
    const leaks: string[] = [];
    for (const { path, method, op } of ops) {
      if (!path.startsWith("/app/")) continue;
      for (const status of Object.keys(op.responses ?? {})) {
        if (!status.startsWith("2")) continue;
        const schema = jsonResponseSchema(op, status);
        if (!schema) continue;
        const names = propertyNames(doc, schema);
        for (const bad of FORBIDDEN) {
          if (!names.has(bad)) continue;
          if (bad === "transcript" && TRANSCRIPT_ALLOWED_AT.has(path)) continue;
          leaks.push(`${method.toUpperCase()} ${path} ${status} exposes '${bad}'`);
        }
      }
    }
    expect(leaks).toEqual([]);
  });

  it("keeps StudentQuestion clean in isolation", () => {
    const names = propertyNames(doc, doc.components.schemas.StudentQuestion);
    for (const bad of FORBIDDEN)
      expect(names.has(bad), `StudentQuestion has ${bad}`).toBe(false);
  });

  it("still gives AdminQuestion the grading key", () => {
    const names = propertyNames(doc, doc.components.schemas.AdminQuestion);
    for (const needed of [
      "isCorrect",
      "sampleAnswer",
      "acceptedAnswers",
      "transcript",
    ]) {
      expect(names.has(needed), `AdminQuestion is missing ${needed}`).toBe(true);
    }
  });
});

describe("§6.5: public endpoints", () => {
  const publicOps = ops.filter(({ op }) => isPublic(op));

  it("finds the public surface", () => {
    expect(publicOps.length).toBeGreaterThanOrEqual(4);
  });

  it.each(publicOps.map((o) => [`${o.method.toUpperCase()} ${o.path}`, o] as const))(
    "%s is rate-limited, tagged public and documents a 429",
    (_label, { path, op }) => {
      const isBeacon = path.endsWith("/events");
      expect(op["x-rate-limit"] ?? isBeacon, "missing x-rate-limit").toBeTruthy();
      if (!isBeacon) {
        expect(op.tags ?? []).toContain("public");
        expect(Object.keys(op.responses ?? {})).toContain("429");
      }
    },
  );
});

describe("conventions", () => {
  it("gives every operation a unique operationId", () => {
    const seen = new Map<string, string>();
    const dupes: string[] = [];
    for (const { path, method, op } of ops) {
      const id = op.operationId;
      expect(id, `${method.toUpperCase()} ${path} has no operationId`).toBeTruthy();
      if (seen.has(id))
        dupes.push(`${id}: ${seen.get(id)} and ${method.toUpperCase()} ${path}`);
      seen.set(id, `${method.toUpperCase()} ${path}`);
    }
    expect(dupes).toEqual([]);
  });

  it("uses one {items, page, pageSize, total} envelope on every paginated list", () => {
    const bad: string[] = [];
    for (const { path, method, op } of ops) {
      const params: Json[] = op.parameters ?? [];
      const paginated = params.some(
        (p) => p?.$ref?.endsWith("/Page") || p?.name === "page",
      );
      if (!paginated) continue;
      const names = propertyNames(doc, jsonResponseSchema(op, 200));
      for (const key of ["items", "page", "pageSize", "total"]) {
        if (!names.has(key)) bad.push(`${method.toUpperCase()} ${path} lacks ${key}`);
      }
    }
    expect(bad).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// The checkers, checked. Each of these builds a document that SHOULD fail and
// asserts that it does — so none of the suites above can quietly become a
// no-op after a refactor.
// ---------------------------------------------------------------------------

describe("the checkers themselves", () => {
  it("propertyNames finds a key nested behind refs, arrays and allOf", () => {
    const synthetic = {
      components: {
        schemas: {
          Leaf: { type: "object", properties: { isCorrect: { type: "boolean" } } },
          Mid: { type: "array", items: { $ref: "#/components/schemas/Leaf" } },
          Root: {
            allOf: [
              {
                type: "object",
                properties: { nested: { $ref: "#/components/schemas/Mid" } },
              },
            ],
          },
        },
      },
    };
    const names = propertyNames(synthetic, synthetic.components.schemas.Root);
    expect(names.has("isCorrect")).toBe(true);
  });

  it("propertyNames terminates on a circular $ref", () => {
    const synthetic = {
      components: {
        schemas: {
          Node: {
            type: "object",
            properties: {
              child: { $ref: "#/components/schemas/Node" },
              id: { type: "string" },
            },
          },
        },
      },
    };
    const names = propertyNames(synthetic, synthetic.components.schemas.Node);
    expect(names.has("id")).toBe(true);
  });

  it("resolveRef reports a dangling reference", () => {
    expect(resolveRef({ components: {} }, "#/components/schemas/Nope")).toBeUndefined();
  });

  it("isPublic recognises both forms of an unauthenticated operation", () => {
    expect(isPublic({ security: [] })).toBe(true);
    expect(isPublic({ security: [{}] })).toBe(true);
    expect(isPublic({ security: [{ bearerAuth: [] }] })).toBe(false);
    expect(isPublic({})).toBe(false);
  });

  it("operations() ignores non-method keys on a path item", () => {
    const found = operations({
      paths: { "/x": { parameters: [{ name: "id" }], get: { operationId: "getX" } } },
    });
    expect(found.map((o) => o.method)).toEqual(["get"]);
  });
});

describe("§11.1's upload limits agree across the layers", () => {
  const asset = resolveRef(doc, "#/components/schemas/MediaAsset") as Record<
    string,
    Json
  >;
  const properties = asset["properties"] as Record<string, Record<string, Json>>;

  it("MAX_BYTES matches MediaAsset.bytes.maximum", () => {
    expect(properties["bytes"]?.["maximum"]).toBe(MAX_BYTES);
  });

  it("MAX_DURATION_MS matches MediaAsset.durationMs.maximum", () => {
    expect(properties["durationMs"]?.["maximum"]).toBe(MAX_DURATION_MS);
  });
});
