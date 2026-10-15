import { describe, expect, it } from "vitest";
import { API_GROUPS } from "@/features/api-reference/groups";
import en from "@/lib/i18n/locales/en.json";
import vi from "@/lib/i18n/locales/vi.json";
import { loadSpec } from "@tests/support/openapi";

type Words = Record<string, { name?: string; blurb?: string }>;

const SPEC = loadSpec();
const TAGS = (SPEC.tags as { name: string }[]).map(({ name }) => name);
const HTTP = ["get", "put", "post", "delete", "patch"];

function pathsOf(tag: string): string[] {
  return Object.entries(
    SPEC.paths as Record<string, Record<string, { tags?: string[] }>>,
  )
    .filter(([, item]) =>
      HTTP.some((method) => item[method]?.tags?.includes(tag) === true),
    )
    .map(([path]) => path);
}
const WORDS: [string, Words][] = [
  ["vi", vi.apiReference.groups as Words],
  ["en", en.apiReference.groups as Words],
];

describe("the API reference's groups follow the contract's tags", () => {
  it("reads the tags the contract lists", () => {
    expect(TAGS.length).toBeGreaterThan(20);
    expect(TAGS).toContain("teacher-tests");
  });

  it.each(TAGS)("%s has an icon and a path", (tag) => {
    const group = API_GROUPS[tag];
    expect(group, `add ${tag} to API_GROUPS`).toBeDefined();
    expect(group!.path.startsWith("/")).toBe(true);
  });

  it.each(TAGS)("%s shows a path that some operation of the tag has", (tag) => {
    const prefix = API_GROUPS[tag]!.path.replace(/\/\*$/, "");
    expect(pathsOf(tag).some((path) => path.startsWith(prefix))).toBe(true);
  });

  it.each(WORDS)("%s has a name and a blurb for every tag", (_, words) => {
    const missing = TAGS.filter(
      (tag) => !words[tag]?.name?.trim() || !words[tag]?.blurb?.trim(),
    );
    expect(missing).toEqual([]);
  });

  it.each(WORDS)("%s names no tag the contract has dropped", (_, words) => {
    expect(Object.keys(words).filter((tag) => !TAGS.includes(tag))).toEqual([]);
  });

  it("draws no group for a tag the contract has dropped", () => {
    expect(Object.keys(API_GROUPS).filter((tag) => !TAGS.includes(tag))).toEqual([]);
  });
});
