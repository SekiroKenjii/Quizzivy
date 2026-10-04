import { describe, expect, it } from "vitest";
import {
  matchSpan,
  TAG_OPTION_LIMIT,
  tagOptions,
  type TagSuggestion,
} from "@/components/shared/form/tagOptions";

const KNOWN: readonly TagSuggestion[] = [
  { tag: "grammar", meta: "2 câu hỏi" },
  { tag: "reading", meta: "2 câu hỏi" },
  { tag: "present perfect", meta: "1 câu hỏi" },
  { tag: "prepositions", meta: "1 câu hỏi" },
  { tag: "Nghé con" },
  { tag: "reported speech" },
  { tag: "Đọc hiểu" },
];

const names = (draft: string, chosen: readonly string[] = [], known = KNOWN) =>
  tagOptions(draft, chosen, known).map((option) =>
    option.create ? `+${option.tag}` : option.tag,
  );

describe("the options a tag field lists", () => {
  it("are the suggestions that contain the draft, then the row that creates it", () => {
    expect(names("re")).toEqual([
      "reading",
      "reported speech",
      "present perfect",
      "prepositions",
      "+re",
    ]);
  });

  it("put those that start with the draft first and keep the order given otherwise", () => {
    expect(names("p")).toEqual([
      "present perfect",
      "prepositions",
      "reported speech",
      "+p",
    ]);
    expect(names("e")).toEqual([
      "reading",
      "present perfect",
      "prepositions",
      "Nghé con",
      "reported speech",
      "Đọc hiểu",
      "+e",
    ]);
  });

  it("match without case and accents", () => {
    expect(names("NGHE")).toEqual(["Nghé con", "+NGHE"]);
    expect(names("doc")).toEqual(["Đọc hiểu", "+doc"]);
    expect(names("nghé")).toEqual(["Nghé con", "+nghé"]);
  });

  it("leave out a tag that is already chosen, however it is spelt", () => {
    expect(names("re", ["Reading"])).toEqual([
      "reported speech",
      "present perfect",
      "prepositions",
      "+re",
    ]);
    expect(names("ngh", ["nghe con"])).toEqual(["+ngh"]);
  });

  it("list six suggestions at most, and the creating row after them", () => {
    const many = Array.from({ length: 9 }, (_, index) => ({
      tag: `unit ${index + 1}`,
    }));
    const listed = names("unit", [], many);
    expect(TAG_OPTION_LIMIT).toBe(6);
    expect(listed).toEqual([
      "unit 1",
      "unit 2",
      "unit 3",
      "unit 4",
      "unit 5",
      "unit 6",
      "+unit",
    ]);
  });

  it("count a later exact match even when six others come before it", () => {
    const many = [
      ...Array.from({ length: 6 }, (_, index) => ({ tag: `a unit ${index + 1}` })),
      { tag: "Unit" },
    ];
    expect(names("unit", [], many)).toEqual([
      "Unit",
      "a unit 1",
      "a unit 2",
      "a unit 3",
      "a unit 4",
      "a unit 5",
    ]);
  });

  it("have no creating row when a suggestion equals the draft without case and accents", () => {
    expect(names("READING")).toEqual(["reading"]);
    expect(names("nghe con")).toEqual(["Nghé con"]);
    expect(names(" reading ")).toEqual(["reading"]);
  });

  it("have no creating row when a chosen tag equals the draft", () => {
    expect(names("Reading", ["reading"])).toEqual([]);
    expect(names("moi", ["Mới"])).toEqual([]);
  });

  it("are none for a blank draft", () => {
    expect(names("")).toEqual([]);
    expect(names("   ")).toEqual([]);
  });

  it("list two suggestions that differ only in case or accents once, the first", () => {
    const known = [{ tag: "Reading" }, { tag: "reading" }, { tag: "réading" }];
    expect(names("rea", [], known)).toEqual(["Reading", "+rea"]);
  });

  it("carry each suggestion's note, and none on the creating row", () => {
    const options = tagOptions("re", [], KNOWN);
    expect(options[0]).toEqual({
      tag: "reading",
      create: false,
      meta: "2 câu hỏi",
      match: [0, 2],
    });
    expect(options[1]?.meta).toBeUndefined();
    expect(options.at(-1)).toEqual({ tag: "re", create: true, match: null });
  });

  it("create the draft trimmed and composed", () => {
    expect(tagOptions("  nghé  ", [], []).at(-1)?.tag).toBe("nghé");
  });
});

describe("the part of a suggestion to embolden", () => {
  it("is found on the suggestion's own text, accents and case included", () => {
    expect(matchSpan("Nghé con", "nghe")).toEqual([0, 4]);
    expect("Nghé con".slice(0, 4)).toBe("Nghé");
    expect(matchSpan("present perfect", "re")).toEqual([1, 3]);
    expect(matchSpan("Đọc hiểu", "doc")).toEqual([0, 3]);
    expect(matchSpan("Đọc hiểu", "HIEU")).toEqual([4, 8]);
  });

  it("covers the combining marks of a decomposed suggestion", () => {
    const text = "Nghé con";
    expect(matchSpan(text, "nghe")).toEqual([0, 5]);
    expect(text.slice(0, 5).normalize("NFC")).toBe("Nghé");
    expect(matchSpan(text, "e c")).toEqual([3, 7]);
    expect(matchSpan("Nghé con", "nghé")).toEqual([0, 4]);
  });

  it("is on every listed suggestion and never on the creating row", () => {
    const options = tagOptions("nghe", [], KNOWN);
    expect(options.map((option) => option.match)).toEqual([[0, 4], null]);
    expect(options[1]?.create).toBe(true);
  });

  it("is nothing for a blank draft or a draft that does not occur", () => {
    expect(matchSpan("reading", "")).toBeNull();
    expect(matchSpan("reading", "  ")).toBeNull();
    expect(matchSpan("reading", "xyz")).toBeNull();
  });
});
