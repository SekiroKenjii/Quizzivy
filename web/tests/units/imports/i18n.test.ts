import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";
import vi from "@/lib/i18n/locales/vi.json";
import en from "@/lib/i18n/locales/en.json";
import { EDITABLE_TYPES } from "@/features/imports/draft";
import { FINDING_CODES, OBJECT_REASONS } from "@/features/imports/findings";
import {
  IMPORT_STATUSES,
  PROCESSING_STAGES,
  RUN_ERROR_KEYS,
} from "@/features/imports/status";

const FEATURE = resolve(import.meta.dirname, "../../../src/features/imports");

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) return files(full);
    return /\.tsx?$/.test(full) ? [full] : [];
  });
}

function lookup(tree: unknown, key: string): unknown {
  return key
    .split(".")
    .reduce<unknown>(
      (node, part) =>
        typeof node === "object" && node !== null
          ? (node as Record<string, unknown>)[part]
          : undefined,
      tree,
    );
}

function resolves(tree: unknown, key: string): boolean {
  if (typeof lookup(tree, key) === "string") return true;
  return (
    typeof lookup(tree, `${key}_one`) === "string" &&
    typeof lookup(tree, `${key}_other`) === "string"
  );
}

const ROLES = ["exam", "answer_key"] as const;
const TEMPLATED: Record<string, string[]> = {
  "imports.status.": [...IMPORT_STATUSES],
  "imports.rowAction.": IMPORT_STATUSES.flatMap((status) => [status, `${status}Named`]),
  "imports.role.": [...ROLES],
  "imports.upload.": ROLES.flatMap((role) => [
    `${role}Label`,
    `${role}Hint`,
    `${role}Choose`,
    `${role}Zone`,
  ]),
  "imports.type.": [...EDITABLE_TYPES, "unsupported"],
  "imports.origin.": [
    "source_explicit",
    "inferred_structure",
    "defaulted",
    "teacher_entered",
  ],
  "imports.field.": ["type", "prompt", "options", "answer", "points"],
  "imports.severity.": ["blocking", "review", "acknowledged", "info"],
  "imports.findings.codes.": [...FINDING_CODES, "UNKNOWN"].flatMap((code) => [
    `${code}.title`,
    `${code}.help`,
  ]),
  "imports.findings.objectReasons.": [...OBJECT_REASONS],
  "imports.runError.": [...RUN_ERROR_KEYS],
  "imports.reprocess.": ["failed", "cancelled"],
  "imports.processing.stage.": PROCESSING_STAGES.map((stage) => stage.key),
  "imports.processing.state.": ["done", "current", "waiting"],
  "imports.review.readOnly.": IMPORT_STATUSES.filter(
    (status) => status !== "committed" && status !== "needs_review",
  ),
};

const sources = files(FEATURE).map((file) => readFileSync(file, "utf8"));
const literal = [
  ...new Set(
    sources.flatMap((source) =>
      [...source.matchAll(/\bt\(\s*"((?:imports|tests|layout)\.[^"]+)"/g)].map(
        (match) => match[1]!,
      ),
    ),
  ),
];
const templatePrefixes = [
  ...new Set(
    sources.flatMap((source) =>
      [...source.matchAll(/\bt\(\s*`(imports\.[^`$]*)\$\{/g)].map((match) => match[1]!),
    ),
  ),
];
const expanded = Object.entries(TEMPLATED).flatMap(([prefix, suffixes]) =>
  suffixes.map((suffix) => `${prefix}${suffix}`),
);
const entryPoints = [
  "tests.importWord",
  "tests.importHistory",
  "layout.resize.importSource",
];

describe("Word import strings", () => {
  it("covers every templated key the feature builds", () => {
    expect(templatePrefixes.filter((prefix) => !(prefix in TEMPLATED))).toEqual([]);
  });

  it.each([
    ["vi", vi],
    ["en", en],
  ] as const)("resolves every key in %s", (_name, tree) => {
    const keys = [...literal, ...expanded, ...entryPoints];
    expect(keys.length).toBeGreaterThan(200);
    expect(keys.filter((key) => !resolves(tree, key))).toEqual([]);
  });

  it.each([
    ["vi", vi],
    ["en", en],
  ] as const)(
    "starts no %s accessible name without its visible label",
    (_name, tree) => {
      const pairs = [
        ...IMPORT_STATUSES.map((status) => [
          `imports.rowAction.${status}`,
          `imports.rowAction.${status}Named`,
        ]),
        ...ROLES.map((role) => [
          "imports.upload.choose",
          `imports.upload.${role}Choose`,
        ]),
        ["imports.review.useCandidate", "imports.review.useCandidateNamed"],
      ];
      const missing = pairs.filter(([visible, named]) => {
        const label = String(lookup(tree, visible!)).toLowerCase();
        return !String(lookup(tree, named!)).toLowerCase().includes(label);
      });
      expect(missing).toEqual([]);
    },
  );

  it("says Sẵn sàng rà soát for a finished run, never Nhập thành công", () => {
    expect(lookup(vi, "imports.status.needs_review")).toBe("Sẵn sàng rà soát");
    expect(JSON.stringify(lookup(vi, "imports"))).not.toContain("Nhập thành công");
  });
});
