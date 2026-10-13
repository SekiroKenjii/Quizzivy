import type { MatcherFunction } from "@testing-library/react";

/**
 * gapSentence matches the paragraph a choice prompt with one gap draws in the
 * engine, as a screen reader reads it: the prompt with its run of
 * underscores heard as `heard`, "chỗ trống" (a blank, in Vietnamese) until
 * an option is chosen.
 */
export function gapSentence(prompt: string, heard = "chỗ trống"): MatcherFunction {
  const expected = prompt.replace(/_{3,}/, heard);
  return (_content, element) =>
    element?.tagName === "P" &&
    element.textContent?.replace(/\s+/g, " ").trim() === expected;
}
