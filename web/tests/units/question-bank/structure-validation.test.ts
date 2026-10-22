import {
  emptyQuestion,
  issueKey,
  questionSchema,
} from "@/features/question-bank/questionSchema";
import vi from "@/lib/i18n/locales/vi.json";

function choice(
  type: "single_choice" | "multiple_choice" | "true_false",
  correct: number,
) {
  return {
    ...emptyQuestion(),
    type,
    prompt: "Chọn đáp án",
    options: [
      { id: null, text: "A", isCorrect: correct > 0 },
      { id: null, text: "B", isCorrect: correct > 1 },
    ],
  };
}

test.each(["single_choice", "true_false"] as const)(
  "%s rejects zero or two keys",
  (type) => {
    expect(questionSchema.safeParse(choice(type, 0)).success).toBe(false);
    expect(questionSchema.safeParse(choice(type, 1)).success).toBe(true);
    expect(questionSchema.safeParse(choice(type, 2)).success).toBe(false);
  },
);

test("multiple choice accepts multiple keys and true/false requires two options", () => {
  expect(questionSchema.safeParse(choice("multiple_choice", 2)).success).toBe(true);
  const value = choice("true_false", 1);
  value.options.push({ id: null, text: "Không được đề cập", isCorrect: false });
  expect(questionSchema.safeParse(value).success).toBe(false);
});

test.each([0, -1, Number.NaN, Number.POSITIVE_INFINITY, 1.005, 1000000])(
  "rejects invalid points %s without rounding",
  (points) => {
    expect(
      questionSchema.safeParse({ ...choice("single_choice", 1), points }).success,
    ).toBe(false);
  },
);

test.each([0.01, 0.29, 1.5, 999999.99])(
  "accepts exact two-decimal points %s",
  (points) => {
    expect(
      questionSchema.safeParse({ ...choice("single_choice", 1), points }).success,
    ).toBe(true);
  },
);

test("rejects disconnected, duplicate and empty blank answers", () => {
  const value = {
    ...emptyQuestion(),
    type: "fill_blank",
    prompt: "Điền {{1}}",
    options: [],
    blanks: [{ id: null, ordinal: 1, acceptedAnswers: ["câu"], caseSensitive: false }],
  };
  expect(questionSchema.safeParse(value).success).toBe(true);
  expect(questionSchema.safeParse({ ...value, blanks: [] }).success).toBe(false);
  expect(
    questionSchema.safeParse({ ...value, prompt: "Không có chỗ trống" }).success,
  ).toBe(false);
  expect(
    questionSchema.safeParse({ ...value, blanks: [...value.blanks, ...value.blanks] })
      .success,
  ).toBe(false);
  expect(
    questionSchema.safeParse({
      ...value,
      blanks: [{ ...value.blanks[0], acceptedAnswers: [" "] }],
    }).success,
  ).toBe(false);
});

test("every issue the editor can show is a translated key, never zod's own text", () => {
  const cases = [
    { ...choice("single_choice", 1), points: 1000000 },
    { ...choice("single_choice", 1), tags: [""] },
    {
      ...choice("single_choice", 1),
      audio: { maxPlays: 0, allowSeek: true, showTranscriptAfterSubmit: false },
    },
    { ...choice("single_choice", 0), prompt: "" },
  ];
  for (const value of cases) {
    const parsed = questionSchema.safeParse(value);
    expect(parsed.success).toBe(false);
    const key = issueKey(parsed.error!, "questionEditor.saveFailed");
    const text = key
      .split(".")
      .reduce<unknown>(
        (node, part) => (node as Record<string, unknown> | undefined)?.[part],
        vi,
      );
    expect(typeof text, key).toBe("string");
  }
});

describe("the R6 types and settings (T-R6.9a)", () => {
  const opt = (text: string, isCorrect = false) => ({ id: null, text, isCorrect });
  const keys = (value: unknown) =>
    questionSchema.safeParse(value).error?.issues.map((issue) => issue.message) ?? [];

  test("tfng takes exactly three options with one correct", () => {
    const value = {
      ...emptyQuestion(),
      type: "tfng",
      prompt: "Đúng không?",
      options: [opt("True"), opt("False", true), opt("Not given")],
    };
    expect(keys(value)).toEqual([]);
    expect(keys({ ...value, options: value.options.slice(0, 2) })).toContain(
      "questionEditor.errors.tfngCount",
    );
    expect(
      keys({
        ...value,
        options: [opt("True", true), opt("False", true), opt("Not given")],
      }),
    ).toContain("questionEditor.errors.oneCorrect");
  });

  test("matching holds its options unmarked, 2 to 20 items, each keyed, one each without reuse", () => {
    const value = {
      ...emptyQuestion(),
      type: "matching",
      prompt: "Nối",
      options: [opt("a"), opt("b")],
      matchAllowReuse: false,
      matchItems: [
        { id: null, text: "x", correctOption: 0 },
        { id: null, text: "y", correctOption: 1 },
      ],
    };
    expect(keys(value)).toEqual([]);
    const unsaid: Partial<typeof value> = { ...value };
    delete unsaid.matchAllowReuse;
    expect(keys(unsaid)).toContain("questionEditor.errors.matchReuseRequired");
    expect(keys({ ...value, options: [opt("a", true), opt("b")] })).toContain(
      "questionEditor.errors.matchOptionCorrect",
    );
    const twice = [
      { id: null, text: "x", correctOption: 1 },
      { id: null, text: "y", correctOption: 1 },
    ];
    expect(keys({ ...value, matchItems: twice })).toContain(
      "questionEditor.errors.matchOptionTaken",
    );
    expect(keys({ ...value, matchItems: twice, matchAllowReuse: true })).toEqual([]);
    const three = [...value.matchItems, { id: null, text: "z", correctOption: 0 }];
    expect(keys({ ...value, matchItems: three })).toContain(
      "questionEditor.errors.matchMoreItemsThanOptions",
    );
    expect(
      keys({ ...value, matchItems: [{ id: null, text: "x", correctOption: 2 }] }),
    ).toEqual(
      expect.arrayContaining([
        "questionEditor.errors.matchTwoItems",
        "questionEditor.errors.matchItemUnset",
      ]),
    );
    const many = Array.from({ length: 21 }, (_, i) => ({
      id: null,
      text: `i${i}`,
      correctOption: 0,
    }));
    expect(keys({ ...value, matchItems: many, matchAllowReuse: true })).toContain(
      "questionEditor.errors.maxMatchItems",
    );
    expect(
      keys({ ...value, options: Array.from({ length: 13 }, (_, i) => opt(`o${i}`)) }),
    ).toContain("questionEditor.errors.maxMatchOptions");
  });

  test("a choice type still stops at eight options", () => {
    expect(
      keys({
        ...emptyQuestion(),
        prompt: "p",
        options: Array.from({ length: 9 }, (_, i) => opt(`o${i}`, i === 0)),
      }),
    ).toContain("questionEditor.errors.maxOptions");
  });

  test("each setting belongs to its own type", () => {
    const single = {
      ...emptyQuestion(),
      prompt: "p",
      options: [opt("a", true), opt("b")],
    };
    for (const [field, extra] of [
      ["selectCount", { selectCount: 2 }],
      ["maxWords", { maxWords: 3 }],
      ["matchAllowReuse", { matchAllowReuse: false }],
      ["matchItems", { matchItems: [{ id: null, text: "x", correctOption: 0 }] }],
      ["rubric", { rubric: [{ id: null, label: "Ideas" }] }],
    ] as const) {
      const parsed = questionSchema.safeParse({ ...single, ...extra });
      expect(parsed.error?.issues.map((issue) => issue.path[0])).toContain(field);
    }
  });

  test("choose N runs from 2 to 7, below the option count, with N ticks", () => {
    const options = [opt("a", true), opt("b", true), opt("c"), opt("d")];
    const value = {
      ...emptyQuestion(),
      type: "multiple_choice",
      prompt: "p",
      options,
      selectCount: 2,
    };
    expect(keys(value)).toEqual([]);
    expect(keys({ ...value, selectCount: 3 })).toContain(
      "questionEditor.errors.selectCountTicks",
    );
    for (const selectCount of [1, 4, 8])
      expect(keys({ ...value, selectCount })).toContain(
        "questionEditor.errors.selectCountRange",
      );
  });

  test("an essay's rubric has at most six distinct, named criteria", () => {
    const value = {
      ...emptyQuestion(),
      type: "essay",
      prompt: "Viết",
      options: [],
      rubric: [{ id: null, label: "Ideas" }],
    };
    expect(keys(value)).toEqual([]);
    expect(
      keys({ ...value, rubric: [...value.rubric, { id: null, label: " ideas " }] }),
    ).toContain("questionEditor.errors.duplicateCriterion");
    expect(keys({ ...value, rubric: [{ id: null, label: " " }] })).toContain(
      "questionEditor.errors.criterionRequired",
    );
    expect(keys({ ...value, rubric: [{ id: null, label: "x".repeat(81) }] })).toContain(
      "questionEditor.errors.criterionTooLong",
    );
    expect(
      keys({
        ...value,
        rubric: Array.from({ length: 7 }, (_, i) => ({ id: null, label: `c${i}` })),
      }),
    ).toContain("questionEditor.errors.maxCriteria");
  });

  test("every new error key has its vi text", () => {
    const errors = (vi as { questionEditor: { errors: Record<string, string> } })
      .questionEditor.errors;
    for (const key of [
      "tfngCount",
      "matchTwoItems",
      "maxMatchItems",
      "maxMatchOptions",
      "matchItemRequired",
      "matchItemTooLong",
      "matchItemUnset",
      "matchOptionTaken",
      "matchMoreItemsThanOptions",
      "matchOptionCorrect",
      "matchReuseRequired",
      "selectCountRange",
      "selectCountTicks",
      "maxWordsRange",
      "unexpectedTypeField",
      "criterionRequired",
      "criterionTooLong",
      "duplicateCriterion",
      "maxCriteria",
    ])
      expect(errors[key], key).toBeTruthy();
  });
});
