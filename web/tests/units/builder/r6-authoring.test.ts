import { describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import type { AdminQuestion } from "@/features/question-bank/api";
import { BANK_TYPES } from "@/features/question-bank/pages/teacher/bankFilters";
import type { GroupBundle } from "@/features/question-groups/api";
import {
  emptyGroup,
  hasOptionOrderChoice,
  memberOptionOrder,
  withMembers,
} from "@/features/question-groups/model";
import { readGroupRecovery } from "@/features/question-groups/recovery";
import { publishProblem } from "@/features/tests/publishProblem";

const A = "0190a000-0000-7000-8000-000000000001";
const B = "0190a000-0000-7000-8000-000000000002";

function question(over: Partial<AdminQuestion>): AdminQuestion {
  return {
    id: "0190a000-0000-7000-8000-0000000000aa",
    type: "single_choice",
    level: null,
    skill: null,
    prompt: "p",
    points: 1,
    tags: [],
    ...over,
  } as AdminQuestion;
}

const t = i18n.getFixedT("en");

describe("the outline's publish problem for the R6 types (T-R6.9b)", () => {
  it("reads No correct answer set for a tfng with no answer", () => {
    expect(
      publishProblem(
        question({
          type: "tfng",
          options: [
            { id: A, ordinal: 0, text: "True", isCorrect: false },
            { id: B, ordinal: 1, text: "False", isCorrect: false },
          ],
        } as Partial<AdminQuestion>),
        t,
      ),
    ).toBe("No correct answer set");
  });

  it("reads it for a choose-N whose ticks are not N, and not for a plain multiple choice", () => {
    const options = [
      { id: A, ordinal: 0, text: "a", isCorrect: true },
      { id: B, ordinal: 1, text: "b", isCorrect: false },
    ];
    const multi = { type: "multiple_choice", options } as Partial<AdminQuestion>;
    expect(publishProblem(question({ ...multi, selectCount: 2 }), t)).toBe(
      "No correct answer set",
    );
    expect(publishProblem(question(multi), t)).toBeNull();
  });

  it("reads it for a matching question with no items or an item keyed to no option of its own", () => {
    const options = [
      { id: A, ordinal: 0, text: "cat", isCorrect: false },
      { id: B, ordinal: 1, text: "dog", isCorrect: false },
    ];
    const item = (correctOptionId: string) => ({
      id: A,
      ordinal: 0,
      key: "A",
      text: "meow",
      correctOptionId,
    });
    const matching = { type: "matching", options } as Partial<AdminQuestion>;
    expect(publishProblem(question({ ...matching, matchItems: [] }), t)).toBe(
      "No correct answer set",
    );
    expect(
      publishProblem(
        question({
          ...matching,
          matchItems: [item("0190a000-0000-7000-8000-0000000000ff")],
        }),
        t,
      ),
    ).toBe("No correct answer set");
    expect(
      publishProblem(question({ ...matching, matchItems: [item(B)] }), t),
    ).toBeNull();
  });

  it("says the same in vi, and leaves an essay and a short answer alone", () => {
    expect(
      publishProblem(question({ type: "tfng", options: [] }), i18n.getFixedT("vi")),
    ).toBe("Chưa có đáp án đúng");
    expect(publishProblem(question({ type: "essay" }), t)).toBeNull();
    expect(
      publishProblem(question({ type: "short_answer", maxWords: 3 }), t),
    ).toBeNull();
  });
});

describe("the bank's type filter", () => {
  it("offers every stored type, tfng after True / False and essay and matching last", () => {
    expect(BANK_TYPES).toEqual([
      "single_choice",
      "multiple_choice",
      "true_false",
      "tfng",
      "fill_blank",
      "short_answer",
      "essay",
      "matching",
    ]);
  });
});

describe("group members of the R6 types", () => {
  it("keeps tfng and matching options fixed, gives the others shuffle, and lets only choice types choose", () => {
    expect(memberOptionOrder("single_choice", "fixed")).toBe("fixed");
    expect(memberOptionOrder("multiple_choice", "shuffle")).toBe("shuffle");
    expect(memberOptionOrder("tfng", "shuffle")).toBe("fixed");
    expect(memberOptionOrder("matching", "shuffle")).toBe("fixed");
    expect(memberOptionOrder("essay", "fixed")).toBe("shuffle");
    expect(hasOptionOrderChoice("true_false")).toBe(true);
    expect(hasOptionOrderChoice("tfng")).toBe(false);
    expect(hasOptionOrderChoice("matching")).toBe(false);
  });

  it("adds a matching member with a fixed order", () => {
    const id = crypto.randomUUID();
    const bundle = withMembers(emptyGroup("Reading"), [
      {
        id,
        input: {
          type: "matching",
          level: null,
          skill: null,
          prompt: "Nối",
          points: 1,
          options: [
            { text: "cat", isCorrect: false },
            { text: "dog", isCorrect: false },
          ],
          matchAllowReuse: false,
          matchItems: [
            { text: "meow", correctOption: 0 },
            { text: "woof", correctOption: 1 },
          ],
        },
      },
    ]);
    expect(bundle.group.members).toEqual([{ questionId: id, optionOrder: "fixed" }]);
  });

  it("recovers a local draft of an essay and a matching member with their settings", () => {
    const essay = crypto.randomUUID();
    const matching = crypto.randomUUID();
    const bundle: GroupBundle = emptyGroup("Writing");
    bundle.questions = [
      {
        id: essay,
        input: {
          type: "essay",
          level: null,
          skill: null,
          prompt: "Viết",
          points: 9,
          rubric: [{ id: null, label: "Ideas" }],
        },
      },
      {
        id: matching,
        input: {
          type: "matching",
          level: null,
          skill: null,
          prompt: "Nối",
          points: 1,
          options: [
            { text: "cat", isCorrect: false },
            { text: "dog", isCorrect: false },
          ],
          matchAllowReuse: true,
          matchItems: [{ id: null, text: "meow", correctOption: 1 }],
        },
      },
    ];
    bundle.group.members = [
      { questionId: essay, optionOrder: "shuffle" },
      { questionId: matching, optionOrder: "fixed" },
    ];
    const recovered = readGroupRecovery(
      { version: 1, revision: 2, bundle },
      bundle.group.id,
    );
    expect(recovered?.bundle.questions[0]!.input.rubric).toEqual([
      { id: null, label: "Ideas" },
    ]);
    expect(recovered?.bundle.questions[1]!.input).toMatchObject({
      matchAllowReuse: true,
      matchItems: [{ id: null, text: "meow", correctOption: 1 }],
    });
  });
});
