import { describe, expect, it } from "vitest";
import { contentPlainText } from "@/components/shared/content/plainText";
import { sentenceGap } from "@/features/take-test/sentenceGap";
import type { StudentQuestion } from "@/features/take-test/api";

type Prompted = Pick<StudentQuestion, "type" | "prompt" | "promptContent">;

function single(prompt: string, over: Partial<Prompted> = {}): Prompted {
  return { type: "single_choice", prompt, ...over };
}

type Rich = NonNullable<Prompted["promptContent"]>;
type Block = Rich["blocks"][number];
type Inline = Extract<Block, { type: "paragraph" }>["content"][number];

const RICH = {
  format: "semantic_v1",
  blocks: [{ type: "paragraph", content: [{ type: "text", text: "She ___ here." }] }],
} as unknown as Rich;

function text(value: string, marks: Extract<Inline, { type: "text" }>["marks"] = []) {
  return { type: "text", text: value, marks } as const;
}

function paragraph(...content: Inline[]): Block {
  return { type: "paragraph", content };
}

function image(alt: string): Block {
  return {
    type: "image",
    assetId: "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b",
    alt,
  } as unknown as Block;
}

function rich(...blocks: Block[]): Prompted {
  const promptContent: Rich = { format: "semantic_v1", blocks };
  return single(contentPlainText(promptContent), { promptContent });
}

describe("a choice sentence with its gap", () => {
  it("splits a single-choice prompt around its one run of underscores", () => {
    expect(sentenceGap(single("If I ___ you, I would study harder."))).toEqual({
      before: "If I ",
      after: " you, I would study harder.",
    });
  });

  it("takes a true/false prompt too", () => {
    expect(
      sentenceGap({ type: "true_false", prompt: "The sun ___ in the east." }),
    ).toEqual({ before: "The sun ", after: " in the east." });
  });

  it("counts a longer run as one gap", () => {
    expect(sentenceGap(single("The letter ________ yesterday."))).toEqual({
      before: "The letter ",
      after: " yesterday.",
    });
  });

  it("puts the gap at either end of the sentence", () => {
    expect(sentenceGap(single("He asked me where I ___."))).toEqual({
      before: "He asked me where I ",
      after: ".",
    });
    expect(sentenceGap(single("___ is my best friend."))).toEqual({
      before: "",
      after: " is my best friend.",
    });
  });

  it("keeps a Vietnamese sentence's diacritics on both sides of the gap", () => {
    expect(sentenceGap(single("Cô ấy ___ ở Hà Nội từ năm 2019."))).toEqual({
      before: "Cô ấy ",
      after: " ở Hà Nội từ năm 2019.",
    });
    expect(sentenceGap(single("Đừng quên ______ (mang) áo mưa nhé!"))).toEqual({
      before: "Đừng quên ",
      after: " (mang) áo mưa nhé!",
    });
  });

  it("composes a decomposed Vietnamese prompt, as its Markdown would", () => {
    const decomposed = "Tôi ___ học hôm qua.".normalize("NFD");
    expect(sentenceGap(single(decomposed))).toEqual({
      before: "Tôi ".normalize("NFC"),
      after: " học hôm qua.".normalize("NFC"),
    });
  });

  it("reads a prompt with a trailing line break as one line", () => {
    expect(sentenceGap(single("She ___ here.\n"))).toEqual({
      before: "She ",
      after: " here.",
    });
  });

  it("reads a rich prompt of one plain paragraph as its text", () => {
    expect(sentenceGap(rich(paragraph(text("She ___ here."))))).toEqual({
      before: "She ",
      after: " here.",
    });
  });

  it("reads a plain paragraph split over several unmarked text nodes", () => {
    expect(
      sentenceGap(rich(paragraph(text("If I "), text("___"), text(" you.")))),
    ).toEqual({ before: "If I ", after: " you." });
  });

  it("takes a rich true/false prompt too", () => {
    expect(
      sentenceGap({
        ...rich(paragraph(text("The sun ___ in the east."))),
        type: "true_false",
      }),
    ).toEqual({ before: "The sun ", after: " in the east." });
  });

  it("keeps a Vietnamese rich prompt's diacritics on both sides of the gap", () => {
    expect(
      sentenceGap(rich(paragraph(text("Cô ấy ___ ở Hà Nội từ năm 2019.")))),
    ).toEqual({
      before: "Cô ấy ",
      after: " ở Hà Nội từ năm 2019.",
    });
  });

  it("composes a decomposed Vietnamese rich prompt", () => {
    const decomposed = "Tôi ___ học hôm qua.".normalize("NFD");
    expect(sentenceGap(rich(paragraph(text(decomposed))))).toEqual({
      before: "Tôi ".normalize("NFC"),
      after: " học hôm qua.".normalize("NFC"),
    });
  });

  it("keeps a straight or curly apostrophe and other plain punctuation", () => {
    expect(
      sentenceGap(single("We couldn’t go out ___ the heavy rain.")),
    ).not.toBeNull();
    expect(sentenceGap(single("It's 5 + 3 = ___ (eight) | nine?"))).not.toBeNull();
  });
});

describe("a prompt that prints as written", () => {
  it.each([
    ["two runs", single("She ___ to school and ___ home.")],
    ["two runs in Vietnamese", single("Tôi ___ đi học và ___ về nhà.")],
    [
      "rich content that does not validate",
      single("She ___ here.", { promptContent: RICH }),
    ],
    [
      "a rich bold mark",
      rich(paragraph(text("She "), text("always", ["bold"]), text(" ___ here."))),
    ],
    [
      "a rich italic gap",
      rich(paragraph(text("She "), text("___", ["italic"]), text(" here."))),
    ],
    [
      "a rich link",
      rich(
        paragraph(
          text("See "),
          { type: "link", href: "https://example.com", content: [text("the notice")] },
          text(" and ___."),
        ),
      ),
    ],
    [
      "a rich line break",
      rich(paragraph(text("Read it."), { type: "break" }, text("She ___ here."))),
    ],
    [
      "a rich gap node",
      rich(
        paragraph(
          text("She "),
          { type: "gap", id: "g1", label: "1" },
          text(" ___ here."),
        ),
      ),
    ],
    [
      "two rich paragraphs",
      rich(paragraph(text("Read the sentence.")), paragraph(text("She ___ here."))),
    ],
    [
      "a rich heading",
      rich({ type: "heading", level: 2, content: [text("She ___ here.")] }),
    ],
    ["a rich image", rich(image("She ___ here."))],
    [
      "a rich image beside the paragraph",
      rich(paragraph(text("She ___ here.")), image("A map")),
    ],
    [
      "two runs in a rich prompt",
      rich(paragraph(text("She ___ to school and ___ home."))),
    ],
    [
      "two runs in a Vietnamese rich prompt",
      rich(paragraph(text("Tôi ___ đi học và ___ về nhà."))),
    ],
    ["a rich prompt with no run", rich(paragraph(text("Choose the best answer.")))],
    [
      "another marker in a rich prompt's text",
      rich(paragraph(text("Gõ tên_tệp rồi điền ___."))),
    ],
    [
      "a rich prompt whose projection is not the prompt",
      { ...rich(paragraph(text("She ___ here."))), prompt: "She ___ there." },
    ],
    [
      "a rich multiple choice",
      { ...rich(paragraph(text("She ___ here."))), type: "multiple_choice" as const },
    ],
    [
      "legacy Markdown content",
      single("She ___ here.", {
        promptContent: {
          format: "legacy_markdown_v1",
          markdown: "She ___ here.",
        } as unknown as Rich,
      }),
    ],
    ["a prompt of several lines", single("Read the sentence.\nShe ___ here.")],
    ["a prompt of several lines, CRLF", single("Read the sentence.\r\nShe ___ here.")],
    ["multiple choice", single("She ___ here.", { type: "multiple_choice" })],
    ["a fill-in", single("She ___ here.", { type: "fill_blank" })],
    ["a short answer", single("She ___ here.", { type: "short_answer" })],
    ["a run of two underscores", single("She __ here.")],
    ["no run at all", single("Choose the word that has a different stress pattern.")],
    ["another underscore", single("Gõ tên_tệp rồi điền ___.")],
    ["a run of two beside the gap", single("She ___ here __ today.")],
    ["emphasis", single("She **always** ___ here.")],
    ["underscore emphasis", single("_She_ ___ here.")],
    ["code", single("Type `ls` then ___.")],
    ["strikethrough", single("She ~~was~~ ___ here.")],
    ["a link", single("See [the notice](https://example.com) and ___.")],
    ["HTML", single("She <b>never</b> ___ here.")],
    ["an escape", single("She \\___ here.")],
    ["an entity", single("Tom &amp; Jerry ___ friends.")],
    ["a heading", single("# She ___ here.")],
    ["a list", single("- She ___ here.")],
    ["a numbered list", single("1. She ___ here.")],
    ["a numbered list, closing paren", single("2) Cô ấy ___ ở đây.")],
  ])("%s", (_name, question) => {
    expect(sentenceGap(question)).toBeNull();
  });
});
