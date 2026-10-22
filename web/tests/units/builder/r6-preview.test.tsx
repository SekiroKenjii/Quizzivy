import "@/lib/i18n";
import { render, screen, within } from "@testing-library/react";
import type { AdminQuestion } from "@/features/question-bank/api";
import type { GroupBundle } from "@/features/question-groups/api";
import { emptyGroup, memberOptionOrder } from "@/features/question-groups/model";
import { asStudent, groupPreview } from "@/features/question-groups/preview";
import {
  independentBundle,
  readGroupRecovery,
} from "@/features/question-groups/recovery";

import { StudentPreview } from "@/features/tests/components/StudentPreview";
import { draftUnkeyed, publishProblem } from "@/features/tests/publishProblem";
import i18n from "@/lib/i18n";
import type { QuestionValues } from "@/features/question-bank/questionSchema";

const t = i18n.getFixedT("en");
const SECTION = "0190a000-0000-7000-8000-0000000000c1";
const A = "0190a000-0000-7000-8000-000000000001";
const B = "0190a000-0000-7000-8000-000000000002";
const C = "0190a000-0000-7000-8000-000000000003";

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

const options = [
  { id: A, ordinal: 0, text: "cat", isCorrect: true },
  { id: B, ordinal: 1, text: "dog", isCorrect: true },
  { id: C, ordinal: 2, text: "bird", isCorrect: false },
];

const matching = question({
  id: "0190a000-0000-7000-8000-0000000000a1",
  type: "matching",
  prompt: "Match the sounds",
  options: options.map((option) => ({ ...option, isCorrect: false })),
  matchAllowReuse: true,
  matchItems: [
    { id: A, ordinal: 0, key: "A", text: "meow", correctOptionId: A },
    { id: B, ordinal: 1, key: "B", text: "woof", correctOptionId: B },
  ],
});
const chooseTwo = question({
  id: "0190a000-0000-7000-8000-0000000000a2",
  type: "multiple_choice",
  prompt: "Pick the mammals",
  options,
  selectCount: 2,
});
const shortAnswer = question({
  id: "0190a000-0000-7000-8000-0000000000a3",
  type: "short_answer",
  prompt: "Name the pet",
  maxWords: 3,
});

describe("the builder's draft preview (QC-R6B-1)", () => {
  it("gives a matching question its items without their keys", () => {
    const student = asStudent(SECTION, matching);
    expect(student.matchItems).toEqual([
      { id: A, key: "A", text: "meow" },
      { id: B, key: "B", text: "woof" },
    ]);
    expect(student.matchAllowReuse).toBe(true);
    expect(student.options!.map((option) => option.key)).toEqual(["i", "ii", "iii"]);
    expect(asStudent(SECTION, chooseTwo).options![0]).not.toHaveProperty("key");
    expect(JSON.stringify(student)).not.toMatch(/correctOption|isCorrect/);
  });

  it("tells a choose N and a word limit, and leaves a plain question without them", () => {
    expect(asStudent(SECTION, chooseTwo).selectCount).toBe(2);
    expect(asStudent(SECTION, shortAnswer).maxWords).toBe(3);
    const plain = asStudent(SECTION, question({ type: "essay", rubric: [] }));
    expect(plain).not.toHaveProperty("selectCount");
    expect(plain).not.toHaveProperty("maxWords");
    expect(plain).not.toHaveProperty("matchItems");
    expect(plain).not.toHaveProperty("rubric");
  });

  it("renders the items, the choose N rule and the word limit in the engine", () => {
    render(
      <StudentPreview
        questions={[matching, chooseTwo, shortAnswer].map((q) => asStudent(SECTION, q))}
      />,
    );
    expect(screen.getByText("meow")).toBeInTheDocument();
    expect(screen.getByText("woof")).toBeInTheDocument();
    // QC-R6B-9: each item's choices carry the options' numerals as their names.
    for (const item of ["A meow", "B woof"])
      expect(
        within(screen.getByRole("radiogroup", { name: item }))
          .getAllByRole("radio")
          .map((radio) => radio.getAttribute("aria-label")),
      ).toEqual(["i", "ii", "iii"]);
    expect(screen.getByText(/^Chọn .+ đáp án\.$/)).toBeInTheDocument();
    expect(screen.getByText(/^Viết KHÔNG QUÁ .+ TỪ\.$/)).toBeInTheDocument();
  });
});

describe("the group's student preview (QC-R6B-1)", () => {
  it("copies the R6 rules and gives each matching item its server key", () => {
    const bundle: GroupBundle = emptyGroup("Reading");
    const ids = [crypto.randomUUID(), crypto.randomUUID(), crypto.randomUUID()];
    bundle.questions = [
      {
        id: ids[0]!,
        input: {
          type: "matching",
          level: null,
          skill: null,
          prompt: "Match",
          points: 1,
          options: [
            { text: "cat", isCorrect: false },
            { text: "dog", isCorrect: false },
          ],
          matchAllowReuse: false,
          matchItems: [
            { id: null, text: "meow", correctOption: 0 },
            { id: null, text: "woof", correctOption: 1 },
          ],
        },
      },
      {
        id: ids[1]!,
        input: {
          type: "multiple_choice",
          level: null,
          skill: null,
          prompt: "Pick",
          points: 1,
          selectCount: 2,
          options: [
            { text: "a", isCorrect: true },
            { text: "b", isCorrect: true },
            { text: "c", isCorrect: false },
          ],
        },
      },
      {
        id: ids[2]!,
        input: {
          type: "short_answer",
          level: null,
          skill: null,
          prompt: "Name",
          points: 1,
          maxWords: 4,
          acceptedAnswers: ["cat"],
        },
      },
    ] as GroupBundle["questions"];
    bundle.group.members = ids.map((questionId) => ({
      questionId,
      optionOrder: "shuffle" as const,
    }));
    const [matched, chosen, written] = groupPreview(bundle, []).questions;
    expect(matched!.matchItems).toEqual([
      { id: `${ids[0]}-item-0`, key: "A", text: "meow" },
      { id: `${ids[0]}-item-1`, key: "B", text: "woof" },
    ]);
    expect(matched!.matchAllowReuse).toBe(false);
    expect(matched!.options!.map((option) => option.key)).toEqual(["i", "ii"]);
    expect(chosen!.options![0]).not.toHaveProperty("key");
    expect(chosen!.selectCount).toBe(2);
    expect(written!.maxWords).toBe(4);
    expect(JSON.stringify([matched, chosen, written])).not.toMatch(
      /correctOption|isCorrect|acceptedAnswers/,
    );
  });
});

describe("the outline's unkeyed rule, both halves (QC-R6B-3, QC-R6B-7)", () => {
  it("warns for a tfng with two keys and a choose N with too many ticks", () => {
    const tfng = question({
      type: "tfng",
      options: options.map((option) => ({ ...option, isCorrect: true })),
    });
    expect(publishProblem(tfng, t)).toBe("No correct answer set");
    expect(publishProblem(question({ ...chooseTwo, selectCount: 1 }), t)).toBe(
      "No correct answer set",
    );
  });

  it("warns for one option keyed to two items unless reuse is on", () => {
    const twice = {
      ...matching,
      matchItems: matching.matchItems!.map((item) => ({ ...item, correctOptionId: A })),
    };
    expect(publishProblem(question({ ...twice, matchAllowReuse: false }), t)).toBe(
      "No correct answer set",
    );
    expect(publishProblem(question({ ...twice, matchAllowReuse: true }), t)).toBeNull();
  });
});

describe("the outline's warning from the editor's draft (QC-R6B-2)", () => {
  const draft = (over: Partial<QuestionValues>): QuestionValues =>
    ({
      type: "multiple_choice",
      level: null,
      skill: null,
      prompt: "p",
      points: 1,
      options: [
        { text: "a", isCorrect: true },
        { text: "b", isCorrect: false },
        { text: "c", isCorrect: false },
      ],
      ...over,
    }) as QuestionValues;

  it("reads a choose N, a tfng and a matching draft the save would refuse", () => {
    expect(draftUnkeyed(draft({ selectCount: 2 }))).toBe(true);
    expect(draftUnkeyed(draft({ selectCount: 1 }))).toBe(false);
    expect(draftUnkeyed(draft({}))).toBe(false);
    expect(draftUnkeyed(draft({ type: "tfng" }))).toBe(false);
    expect(
      draftUnkeyed(
        draft({
          type: "tfng",
          options: draft({}).options.map((o) => ({ ...o, isCorrect: true })),
        }),
      ),
    ).toBe(true);
    const items = (keys: number[]) =>
      keys.map((correctOption, index) => ({
        id: null,
        text: `i${index}`,
        correctOption,
      }));
    const match = (over: Partial<QuestionValues>) =>
      draft({ type: "matching", matchAllowReuse: false, ...over });
    expect(draftUnkeyed(match({ matchItems: items([0, 1]) }))).toBe(false);
    expect(draftUnkeyed(match({ matchItems: [] }))).toBe(true);
    expect(draftUnkeyed(match({ matchItems: items([0, -1]) }))).toBe(true);
    expect(draftUnkeyed(match({ matchItems: items([0, 0]) }))).toBe(true);
    expect(
      draftUnkeyed(match({ matchItems: items([0, 0]), matchAllowReuse: true })),
    ).toBe(false);
  });
});

describe("group members across a type round trip (QC-R6B-4)", () => {
  it("starts a choice member from shuffle when it comes back from tfng or matching", () => {
    expect(memberOptionOrder("single_choice", "fixed", "tfng")).toBe("shuffle");
    expect(memberOptionOrder("multiple_choice", "fixed", "matching")).toBe("shuffle");
    expect(memberOptionOrder("single_choice", "fixed", "multiple_choice")).toBe(
      "fixed",
    );
  });
});

describe("recovery and Save as new group (QC-R6B-5, QC-R6B-7)", () => {
  function bundleWith(): GroupBundle {
    const bundle: GroupBundle = emptyGroup("Writing");
    const id = crypto.randomUUID();
    bundle.questions = [
      {
        id,
        input: {
          type: "essay",
          level: null,
          skill: null,
          prompt: "Viết",
          points: 9,
          rubric: [{ id: A, label: "Ideas" }],
        },
      },
      {
        id: crypto.randomUUID(),
        input: {
          type: "matching",
          level: null,
          skill: null,
          prompt: "Nối",
          points: 1,
          options: [
            { id: A, text: "cat", isCorrect: false },
            { id: B, text: "dog", isCorrect: false },
          ],
          matchAllowReuse: false,
          matchItems: [
            { id: B, text: "meow", correctOption: 0 },
            { id: C, text: "woof", correctOption: 1 },
          ],
        },
      },
      {
        id: crypto.randomUUID(),
        input: {
          type: "multiple_choice",
          level: null,
          skill: null,
          prompt: "Pick",
          points: 1,
          selectCount: 2,
          options: [
            { text: "a", isCorrect: true },
            { text: "b", isCorrect: true },
            { text: "c", isCorrect: false },
          ],
        },
      },
      {
        id: crypto.randomUUID(),
        input: {
          type: "short_answer",
          level: null,
          skill: null,
          prompt: "Name",
          points: 1,
          maxWords: 7,
          acceptedAnswers: ["cat"],
        },
      },
    ] as GroupBundle["questions"];
    bundle.group.members = bundle.questions.map((q) => ({
      questionId: q.id,
      optionOrder: "shuffle" as const,
    }));
    return bundle;
  }

  it("restores a choose N and a word limit", () => {
    const bundle = bundleWith();
    const recovered = readGroupRecovery(
      { version: 1, revision: 2, bundle },
      bundle.group.id,
    );
    expect(recovered?.bundle.questions[2]!.input.selectCount).toBe(2);
    expect(recovered?.bundle.questions[3]!.input.maxWords).toBe(7);
  });

  it("drops the original's match item and rubric ids from the copy", () => {
    const copied = independentBundle(bundleWith());
    expect(copied.questions[0]!.input.rubric).toEqual([{ id: null, label: "Ideas" }]);
    expect(copied.questions[1]!.input.matchItems).toEqual([
      { id: null, text: "meow", correctOption: 0 },
      { id: null, text: "woof", correctOption: 1 },
    ]);
  });
});
