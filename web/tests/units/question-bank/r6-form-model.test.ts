import { describe, expect, it } from "vitest";
import { toFormValues, type AdminQuestion } from "@/features/question-bank/api";
import {
  blockingIssue,
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import {
  editorType,
  readTfng,
  retype,
  tfngOptions,
  withNotGiven,
} from "@/features/question-bank/questionType";

type Option = QuestionValues["options"][number];

const TRUE_ID = "0190a000-0000-7000-8000-000000000001";
const FALSE_ID = "0190a000-0000-7000-8000-000000000002";
const ITEM_ID = "0190a000-0000-7000-8000-0000000000aa";

function option(text: string, isCorrect = false, id: string | null = null): Option {
  return { id, text, isCorrect };
}

describe("True / False / Not given in the form", () => {
  const tf: QuestionValues = {
    ...emptyQuestion(),
    type: "true_false",
    prompt: "p",
    options: [option("False", false, FALSE_ID), option("True", true, TRUE_ID)],
  };

  it("shows as True / False and turns into tfng keeping the answer and both ids", () => {
    expect(editorType("tfng")).toBe("true_false");
    const on = withNotGiven(tf, true);
    expect(on.type).toBe("tfng");
    expect(on.options).toEqual([
      { id: TRUE_ID, text: "True", isCorrect: true },
      { id: FALSE_ID, text: "False", isCorrect: false },
      { id: null, text: "Not given", isCorrect: false },
    ]);
    expect(withNotGiven(on, true)).toBe(on);
  });

  it("makes False the answer when Not given was, on the way back", () => {
    const ng = { ...withNotGiven(tf, true), options: tfngOptions("not_given") };
    expect(readTfng(ng.options)).toBe("not_given");
    const off = withNotGiven(ng, false);
    expect(off.type).toBe("true_false");
    expect(off.options.map((entry) => [entry.text, entry.isCorrect])).toEqual([
      ["True", false],
      ["False", true],
    ]);
  });

  it("keeps its answer through retype and reads True with nothing ticked", () => {
    const ng = withNotGiven(tf, true);
    expect(retype(ng, "tfng").options).toEqual(ng.options);
    expect(retype(ng, "true_false").type).toBe("true_false");
    expect(retype({ ...emptyQuestion(), options: [] }, "tfng").options).toHaveLength(3);
    expect(readTfng([])).toBe("true");
  });
});

describe("retype and the R6 settings", () => {
  it("keeps each setting only on its own type", () => {
    const multi: QuestionValues = {
      ...emptyQuestion(),
      type: "multiple_choice",
      prompt: "p",
      options: [option("a", true), option("b", true), option("c")],
      selectCount: 2,
    };
    expect(retype(multi, "multiple_choice").selectCount).toBe(2);
    expect(retype(multi, "single_choice")).not.toHaveProperty("selectCount");
    const short = retype({ ...emptyQuestion(), maxWords: 3 }, "short_answer");
    expect(short).not.toHaveProperty("maxWords");
    expect(retype({ ...short, maxWords: 3 }, "short_answer").maxWords).toBe(3);
    expect(retype({ ...short, sampleAnswer: "m" }, "essay").sampleAnswer).toBe("m");
    expect(
      retype({ ...short, sampleAnswer: "m" }, "fill_blank").sampleAnswer,
    ).toBeNull();
  });

  it("gives an essay nine points from the starter's one and back, and keeps points the teacher set", () => {
    const essay = retype({ ...emptyQuestion(), prompt: "Viết" }, "essay");
    expect(essay).toMatchObject({ type: "essay", points: 9, rubric: [], options: [] });
    expect(retype({ ...emptyQuestion(), points: 4 }, "essay").points).toBe(4);
    expect(retype(essay, "matching").points).toBe(1);
    expect(retype({ ...essay, points: 18 }, "short_answer").points).toBe(18);
    expect(blockingIssue(essay, "x")).toBeNull();
  });

  it("starts matching with two unset items and carries the options unmarked", () => {
    const next = retype(
      { ...emptyQuestion(), options: [option("x", true), option("y")] },
      "matching",
    );
    expect(next.options.map((entry) => entry.isCorrect)).toEqual([false, false]);
    expect(next.matchAllowReuse).toBe(false);
    expect(next.matchItems).toEqual([
      { id: null, text: "", correctOption: -1 },
      { id: null, text: "", correctOption: -1 },
    ]);
    expect(blockingIssue({ ...next, prompt: "p" }, "x")).toBe(
      "questionEditor.errors.matchItemRequired",
    );
    const keyed = {
      ...next,
      prompt: "p",
      matchItems: [
        { id: null, text: "a", correctOption: -1 },
        { id: null, text: "b", correctOption: 1 },
      ],
    };
    expect(blockingIssue(keyed, "x")).toBe("questionEditor.errors.matchItemUnset");
    const back = retype(next, "single_choice");
    expect(back).not.toHaveProperty("matchItems");
    expect(back).not.toHaveProperty("matchAllowReuse");
    expect(back.options.map((entry) => entry.text)).toEqual(["x", "y"]);
  });

  it("blocks a choose-N question whose ticks differ from N", () => {
    const multi: QuestionValues = {
      ...emptyQuestion(),
      type: "multiple_choice",
      prompt: "p",
      options: [option("a", true), option("b"), option("c"), option("d")],
      selectCount: 2,
    };
    expect(blockingIssue(multi, "x")).toBe("questionEditor.errors.selectCountTicks");
  });
});

describe("toFormValues for the R6 types", () => {
  const base = {
    id: ITEM_ID,
    level: null,
    skill: null,
    prompt: "p",
    points: 1,
    tags: [],
  };

  it("names a matching item's option by its index and keeps the reuse switch", () => {
    const values = toFormValues({
      ...base,
      type: "matching",
      options: [
        { id: TRUE_ID, ordinal: 0, text: "cat", isCorrect: false },
        { id: FALSE_ID, ordinal: 1, text: "dog", isCorrect: false },
      ],
      matchAllowReuse: false,
      matchItems: [
        { id: ITEM_ID, ordinal: 0, key: "A", text: "woof", correctOptionId: FALSE_ID },
      ],
    } as unknown as AdminQuestion);
    expect(values.matchItems).toEqual([
      { id: ITEM_ID, text: "woof", correctOption: 1 },
    ]);
    expect(values.matchAllowReuse).toBe(false);
  });

  it("reads an essay's rubric, a choose-N count and a word limit only on their own types", () => {
    const read = (extra: object) =>
      toFormValues({ ...base, ...extra } as unknown as AdminQuestion);
    expect(
      read({ type: "essay", rubric: [{ id: TRUE_ID, ordinal: 0, label: "Ideas" }] })
        .rubric,
    ).toEqual([{ id: TRUE_ID, label: "Ideas" }]);
    expect(read({ type: "multiple_choice", selectCount: 2 }).selectCount).toBe(2);
    expect(read({ type: "short_answer", maxWords: 3 }).maxWords).toBe(3);
    expect(read({ type: "single_choice", maxWords: 3 })).not.toHaveProperty("maxWords");
  });
});
