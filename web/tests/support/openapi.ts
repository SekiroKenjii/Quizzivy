import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { parse } from "yaml";

/**
 * Loads api/openapi.yaml and provides the structural helpers the contract tests
 * use. Kept separate from the tests so the helpers can themselves be tested
 * against synthetic documents — a checker nobody checks is decoration.
 */

// tests/support -> tests -> web -> repo root.
export const SPEC_PATH = resolve(import.meta.dirname, "../../../api/openapi.yaml");

/**
 * An OpenAPI document is an arbitrary JSON tree. Typing it fully would be its
 * own project and would not make these structural walks any safer — they are
 * deliberately shape-agnostic, which is the point (§14 permits `any` with a
 * stated reason; this is it).
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Json = any;

export function loadSpec(): Json {
  return parse(readFileSync(SPEC_PATH, "utf8"));
}

export const HTTP_METHODS = ["get", "post", "put", "patch", "delete"] as const;

/** Every `$ref` string anywhere in the document. */
export function collectRefs(node: Json, out: string[] = []): string[] {
  if (Array.isArray(node)) {
    for (const v of node) collectRefs(v, out);
  } else if (node && typeof node === "object") {
    if (typeof node.$ref === "string") out.push(node.$ref);
    for (const v of Object.values(node)) collectRefs(v, out);
  }
  return out;
}

/** Resolve a local JSON pointer such as `#/components/schemas/User`. */
export function resolveRef(doc: Json, ref: string): Json | undefined {
  if (!ref.startsWith("#/")) return undefined;
  let node: Json = doc;
  for (const raw of ref.slice(2).split("/")) {
    const key = raw.replace(/~1/g, "/").replace(/~0/g, "~");
    if (node === undefined || node === null) return undefined;
    node = node[key];
  }
  return node;
}

/**
 * Every property name reachable from a schema, following $refs, allOf/oneOf/
 * anyOf, items and additionalProperties.
 *
 * Depth matters: a leak is far more likely to be nested three levels down
 * inside an attempt payload than sitting at the top of a response.
 */
export function propertyNames(
  doc: Json,
  node: Json,
  seen = new Set<string>(),
  depth = 0,
): Set<string> {
  const names = new Set<string>();
  if (!node || typeof node !== "object" || depth > 40) return names;

  if (Array.isArray(node)) {
    for (const v of node)
      for (const n of propertyNames(doc, v, seen, depth + 1)) names.add(n);
    return names;
  }

  if (typeof node.$ref === "string") {
    if (seen.has(node.$ref)) return names;
    seen.add(node.$ref);
    return propertyNames(doc, resolveRef(doc, node.$ref), seen, depth + 1);
  }

  if (node.properties && typeof node.properties === "object") {
    for (const [key, value] of Object.entries(node.properties)) {
      names.add(key);
      for (const n of propertyNames(doc, value, seen, depth + 1)) names.add(n);
    }
  }
  for (const key of [
    "items",
    "allOf",
    "oneOf",
    "anyOf",
    "additionalProperties",
    "not",
  ]) {
    if (key in node) {
      for (const n of propertyNames(doc, node[key], seen, depth + 1)) names.add(n);
    }
  }
  return names;
}

export interface Operation {
  path: string;
  method: string;
  op: Json;
}

export function operations(doc: Json): Operation[] {
  const out: Operation[] = [];
  for (const [path, item] of Object.entries(doc.paths ?? {})) {
    for (const method of HTTP_METHODS) {
      const op = (item as Json)[method];
      if (op && typeof op === "object") out.push({ path, method, op });
    }
  }
  return out;
}

/** `security: []`, or a `{}` entry, means the operation is unauthenticated. */
export function isPublic(op: Json): boolean {
  const sec = op.security;
  if (sec === undefined) return false;
  if (Array.isArray(sec) && sec.length === 0) return true;
  return Array.isArray(sec) && sec.some((s: Json) => s && Object.keys(s).length === 0);
}

export function jsonResponseSchema(
  op: Json,
  status: string | number,
): Json | undefined {
  return op?.responses?.[String(status)]?.content?.["application/json"]?.schema;
}
