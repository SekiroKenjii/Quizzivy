import { describe, expect, it } from "vitest";

import { countWords } from "@/lib/words";

describe("countWords", () => {
  it("counts nothing in blank text", () => {
    expect(countWords("")).toBe(0);
    expect(countWords("   \n\t ")).toBe(0);
    expect(countWords(" 　")).toBe(0);
  });

  it("counts whitespace-separated words, however they are spaced", () => {
    expect(countWords("I wake up at six.")).toBe(5);
    expect(countWords("  two \n\n words  ")).toBe(2);
    expect(countWords("tôi đi học")).toBe(3);
  });

  it("counts a hyphenated word once", () => {
    expect(countWords("a well-known city")).toBe(3);
    expect(countWords("mother-in-law")).toBe(1);
  });

  it("counts a number once", () => {
    expect(countWords("3.5 kilometres")).toBe(2);
    expect(countWords("1,000 people in 2024")).toBe(4);
    expect(countWords("15%")).toBe(1);
  });

  it("does not count punctuation standing alone", () => {
    expect(countWords("parks - and trees — and rivers")).toBe(5);
    expect(countWords("• … ?")).toBe(0);
  });

  it("counts words in Vietnamese and in other scripts", () => {
    expect(countWords("Công viên giúp người dân thư giãn")).toBe(7);
    expect(countWords("Đường phố")).toBe(2);
  });
});
