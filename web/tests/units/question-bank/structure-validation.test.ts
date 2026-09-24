import { emptyQuestion, questionSchema } from "@/features/question-bank/questionSchema";

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
