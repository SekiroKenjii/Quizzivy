import { describe, expect, it } from "vitest";
import { loadSpec, type Json } from "@tests/support/openapi";

/**
 * `allOf: [{$ref: Closed}, {properties: {extra}}]` does not work when `Closed`
 * declares `additionalProperties: false`. In JSON Schema 2020-12 that keyword
 * is evaluated inside its own subschema only — it cannot see the sibling allOf
 * branch — so the extra property is "additional" and the whole body is
 * rejected.
 */
describe("no response schema composes allOf over a closed base", () => {
  it("finds none", () => {
    const spec = loadSpec() as Json;
    const schemas = (spec?.components?.schemas ?? {}) as Record<string, Json>;
    const closed = new Set(
      Object.entries(schemas)
        .filter(([, s]) => s?.additionalProperties === false)
        .map(([name]) => name),
    );

    const offenders: string[] = [];

    const walk = (node: Json, path: string): void => {
      if (Array.isArray(node)) {
        node.forEach((child, i) => walk(child, `${path}[${i}]`));
        return;
      }
      if (node === null || typeof node !== "object") return;

      const branches = node["allOf"];
      if (Array.isArray(branches)) {
        const refs = branches
          .filter((b) => typeof b?.["$ref"] === "string")
          .map((b) => String(b["$ref"]).split("/").pop());
        const adds = branches.filter((b) => !b?.["$ref"] && b?.["properties"]);
        for (const ref of refs) {
          if (ref && closed.has(ref) && adds.length > 0) {
            const added = adds.flatMap((b) => Object.keys(b["properties"] ?? {}));
            offenders.push(`${path}: allOf over ${ref} adds ${added.join(", ")}`);
          }
        }
      }
      for (const [key, child] of Object.entries(node))
        walk(child as Json, `${path}/${key}`);
    };

    walk(spec, "");

    expect(
      offenders,
      "Use a flat standalone schema instead — this file's own convention for a " +
        "shape that differs from the base entity. Do NOT fix it by removing " +
        "additionalProperties: false from the base; that closure is load-bearing " +
        "and tested in msw-contract.test.ts.",
    ).toEqual([]);
  });
});
