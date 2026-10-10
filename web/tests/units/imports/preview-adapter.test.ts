import { describe, expect, it } from "vitest";
import { draftPreview } from "@/features/imports/previewAdapter";
import type { ImportDraftSection } from "@/features/imports/api";
import type { ContentDocument } from "@/components/shared/content/model";
import { option, question, section, text } from "./fixtures";

const ANSWER_KEY = /^(isCorrect|acceptedAnswers|sampleAnswer|correct.*|answer.*)$/i;

function keysOf(value: unknown, path = ""): string[] {
  if (Array.isArray(value))
    return value.flatMap((item, index) => keysOf(item, `${path}[${index}]`));
  if (value === null || typeof value !== "object") return [];
  return Object.entries(value).flatMap(([key, item]) => [
    `${path}.${key}`,
    ...keysOf(item, `${path}.${key}`),
  ]);
}

const gapPrompt: ContentDocument = {
  format: "semantic_v1",
  blocks: [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "She ", marks: [] },
        { type: "gap", id: "g1", label: "1" },
        { type: "text", text: " to school.", marks: [] },
      ],
    },
  ],
};

const passage: ContentDocument = {
  format: "semantic_v1",
  blocks: [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "The city ", marks: [] },
        { type: "gap", id: "p1", label: "5" },
        { type: "text", text: " a park.", marks: [] },
      ],
    },
  ],
};

function everyKind(): ImportDraftSection[] {
  return [
    section(
      [
        question({ id: "single", label: "1" }),
        question({
          id: "multiple",
          label: "2",
          type: "multiple_choice",
          answer: {
            state: "known",
            optionIds: ["multiple-a", "multiple-b"],
            evidence: [],
          },
        }),
        question({
          id: "truefalse",
          label: "3",
          type: "true_false",
          options: [
            option("truefalse-t", "A", "True"),
            option("truefalse-f", "B", "False"),
          ],
          answer: { state: "known", optionIds: ["truefalse-t"], evidence: [] },
        }),
        question({
          id: "blank",
          label: "4",
          type: "fill_blank",
          prompt: gapPrompt,
          options: [],
          blanks: [
            {
              gapId: "g1",
              label: "1",
              accepted: ["went", "walked"],
              caseSensitive: false,
            },
          ],
          answer: { state: "known", optionIds: [], text: "went", evidence: [] },
        }),
        question({
          id: "short",
          label: "6",
          type: "short_answer",
          options: [],
          answer: {
            state: "known",
            optionIds: [],
            text: "a sample answer",
            evidence: [],
          },
        }),
        question({
          id: "dropped",
          label: "7",
          excluded: { reason: "Trùng câu 5" },
        }),
      ],
      { id: "part-1", title: "Phần 1", instructions: "Chọn đáp án đúng." },
    ),
    {
      id: "part-2",
      title: "Phần 2",
      origin: "source_explicit",
      source: [],
      items: [
        {
          group: {
            id: "group-1",
            label: "5",
            instructions: "Đọc đoạn văn.\nChọn từ đúng.",
            stimulus: passage,
            gaps: [{ gapId: "p1", questionId: "member" }],
            source: [],
            questions: [question({ id: "member", label: "5.1" })],
          },
        },
      ],
    },
  ];
}

describe("draftPreview", () => {
  it("never carries the answer key, accepted values or anything only the teacher sees", () => {
    const preview = draftPreview(everyKind());
    const keys = keysOf(preview);
    expect(keys.length).toBeGreaterThan(0);
    expect(keys.filter((key) => ANSWER_KEY.test(key.split(".").at(-1) ?? ""))).toEqual(
      [],
    );
    const json = JSON.stringify(preview);
    for (const secret of [
      "walked",
      "a sample answer",
      "Trùng câu 5",
      "source_explicit",
      "evidence",
      "candidates",
      "origins",
    ])
      expect(json, secret).not.toContain(secret);
  });

  it("keeps every included question in order, with its section, and leaves the excluded one out", () => {
    const preview = draftPreview(everyKind());
    expect(preview.questions.map((item) => item.id)).toEqual([
      "single",
      "multiple",
      "truefalse",
      "blank",
      "short",
      "member",
    ]);
    expect(preview.included).toBe(6);
    expect(preview.sections).toEqual([
      { id: "part-1", title: "Phần 1", instructions: "Chọn đáp án đúng." },
      { id: "part-2", title: "Phần 2", instructions: null },
    ]);
    expect(preview.questions.find((item) => item.id === "blank")?.blanks).toEqual([
      { id: "blank-blank-1", ordinal: 1, gapId: "g1", caseSensitive: false },
    ]);
  });

  it("carries a shared passage with its gaps bound to the question they serve", () => {
    const [group] = draftPreview(everyKind()).groups;
    expect(group?.questionIds).toEqual(["member"]);
    expect(group?.stimuli[0]?.gaps).toEqual([
      { kind: "question", gapId: "p1", questionId: "member" },
    ]);
    expect(group?.instructions?.blocks).toHaveLength(2);
  });

  it("counts a question the paper cannot draw as included but not shown", () => {
    const preview = draftPreview([
      section([
        question({ id: "single", label: "1" }),
        question({ id: "odd", label: "2", type: "unsupported", prompt: text("Drag") }),
      ]),
    ]);
    expect(preview.questions).toHaveLength(1);
    expect(preview.included).toBe(2);
  });
});
