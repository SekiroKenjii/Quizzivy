import { describe, expect, it } from "vitest";
import type { TFunction } from "i18next";
import i18n from "@/lib/i18n";
import {
  chosenWord,
  isRun,
  nextEmptyGap,
  paperStops,
  sharedPrompt,
  stepBack,
  stepOn,
  type Stop,
} from "@/features/take-test/stops";
import type { StudentGroup, StudentQuestion } from "@/features/take-test/api";

type Gap = StudentGroup["stimuli"][number]["gaps"][number];

const choice = (id: string, over: Partial<StudentQuestion> = {}): StudentQuestion => ({
  id,
  sectionId: "s1",
  type: "single_choice",
  prompt: "Choose the best option for each gap.",
  points: 1,
  options: [
    { id: `${id}-a`, text: "relax" },
    { id: `${id}-b`, text: "relaxing" },
  ],
  ...over,
});

const image = {
  id: "018f0000-0000-7000-8000-00000000f001",
  kind: "image",
  url: "https://assets.example/a.png",
  mimeType: "image/png",
} as unknown as NonNullable<StudentQuestion["media"]>;

const audio = {
  id: "018f0000-0000-7000-8000-00000000f002",
  kind: "audio",
  url: "https://assets.example/a.mp3",
  mimeType: "audio/mpeg",
} as unknown as NonNullable<StudentQuestion["media"]>;

const bound = (questionId: string): Gap => ({
  kind: "question",
  gapId: `gap-${questionId}`,
  questionId,
});

function groupOf(questionIds: string[], gaps: Gap[][]): StudentGroup {
  return {
    id: "g1",
    sectionId: "s1",
    title: "Reading",
    questionIds,
    stimuli: gaps.map((list, at) => ({
      id: `m${at}`,
      title: `Material ${at}`,
      content: { format: "semantic_v1", blocks: [] },
      gaps: list,
    })),
    recordings: [],
    assets: [],
  } as unknown as StudentGroup;
}

const shape = (stops: Stop[]) => stops.map((stop) => stop.positions.join("-"));

describe("paperStops", () => {
  it("gathers consecutive bound single-choice members of one material into a run", () => {
    const questions = ["q0", "q1", "q2", "q3", "q4"].map((id) => choice(id));
    const stops = paperStops(questions, [
      groupOf(["q1", "q2", "q3"], [["q1", "q2", "q3"].map(bound)]),
    ]);
    expect(shape(stops)).toEqual(["0", "1-2-3", "1-2-3", "1-2-3", "4"]);
    expect(stops[1]).toBe(stops[3]);
    expect(isRun(stops[2])).toBe(true);
    expect(stops[2]!.cloze?.positions).toEqual([1, 2, 3]);
    expect(isRun(stops[0])).toBe(false);
    expect(stops[0]!.cloze).toBeNull();
  });

  it("breaks a run at a member of another type", () => {
    const questions = [
      choice("q0"),
      choice("q1"),
      choice("q2", { type: "multiple_choice" }),
      choice("q3"),
      choice("q4"),
    ];
    const stops = paperStops(questions, [
      groupOf(
        ["q0", "q1", "q2", "q3", "q4"],
        [["q0", "q1", "q2", "q3", "q4"].map(bound)],
      ),
    ]);
    expect(shape(stops)).toEqual(["0-1", "0-1", "2", "3-4", "3-4"]);
    expect(stops[2]!.cloze).toBeNull();
    expect(stops[0]!.cloze).toBe(stops[3]!.cloze);
    expect(stops[0]!.cloze?.positions).toEqual([0, 1, 3, 4]);
  });

  it("ends a run at a member with an image, which is a stop of its own", () => {
    const questions = [choice("q0"), choice("q1"), choice("q2", { media: image })];
    const stops = paperStops(questions, [
      groupOf(["q0", "q1", "q2"], [["q0", "q1", "q2"].map(bound)]),
    ]);
    expect(shape(stops)).toEqual(["0-1", "0-1", "2"]);
    expect(isRun(stops[2])).toBe(false);
    expect(stops[2]!.cloze?.positions).toEqual([0, 1, 2]);
  });

  it("ends a run at a member with audio, and a gap left alone is a stop of its own", () => {
    const questions = [choice("q0"), choice("q1", { media: audio }), choice("q2")];
    const stops = paperStops(questions, [
      groupOf(["q0", "q1", "q2"], [["q0", "q1", "q2"].map(bound)]),
    ]);
    expect(shape(stops)).toEqual(["0", "1", "2"]);
    expect(stops.every((stop) => !isRun(stop))).toBe(true);
    expect(stops.every((stop) => stop.cloze?.positions.length === 3)).toBe(true);
  });

  it("makes a lone bound member a stop of one question of a cloze of one gap", () => {
    const questions = [choice("q0"), choice("q1")];
    const stops = paperStops(questions, [groupOf(["q0", "q1"], [[bound("q1")]])]);
    expect(shape(stops)).toEqual(["0", "1"]);
    expect(stops[1]!.cloze?.positions).toEqual([1]);
  });

  it("does not gather members bound to two different materials", () => {
    const questions = [choice("q0"), choice("q1")];
    const stops = paperStops(questions, [
      groupOf(["q0", "q1"], [[bound("q0")], [bound("q1")]]),
    ]);
    expect(shape(stops)).toEqual(["0", "1"]);
  });

  it("leaves a gap bound to a blank out of any cloze", () => {
    const questions = [
      choice("q0"),
      choice("q1", { type: "fill_blank", options: [] }),
      choice("q2"),
    ];
    const stops = paperStops(questions, [
      groupOf(
        ["q0", "q1", "q2"],
        [
          [
            bound("q0"),
            { kind: "blank", gapId: "b", questionId: "q1", blankGapId: "x" },
            bound("q2"),
          ],
        ],
      ),
    ]);
    expect(shape(stops)).toEqual(["0", "1", "2"]);
    expect(stops[1]!.cloze).toBeNull();
    expect(stops[0]!.cloze?.positions).toEqual([0, 2]);
  });

  it("ignores a binding to a question outside its group", () => {
    const questions = [choice("q0"), choice("q1")];
    const stops = paperStops(questions, [
      groupOf(["q0"], [[bound("q0"), bound("q1")]]),
    ]);
    expect(shape(stops)).toEqual(["0", "1"]);
    expect(stops[1]!.cloze).toBeNull();
  });

  it("makes every question of a paper with no groups a stop of its own", () => {
    const questions = ["q0", "q1", "q2"].map((id) => choice(id));
    const stops = paperStops(questions, []);
    expect(shape(stops)).toEqual(["0", "1", "2"]);
    expect(stops.every((stop) => stop.cloze === null)).toBe(true);
    expect(paperStops([], [])).toEqual([]);
  });
});

describe("stepping by stop", () => {
  const questions = ["q0", "q1", "q2", "q3", "q4"].map((id) => choice(id));
  const stops = paperStops(questions, [
    groupOf(["q1", "q2", "q3"], [["q1", "q2", "q3"].map(bound)]),
  ]);

  it("steps over a run and lands on its first gap from either side", () => {
    expect(stepOn(stops, 0)).toBe(1);
    expect(stepOn(stops, 2)).toBe(4);
    expect(stepBack(stops, 4)).toBe(1);
    expect(stepBack(stops, 3)).toBe(0);
  });

  it("stops at both ends", () => {
    expect(stepBack(stops, 0)).toBeNull();
    expect(stepOn(stops, 4)).toBeNull();
  });
});

describe("nextEmptyGap", () => {
  const questions = ["q0", "q1", "q2", "q3"].map((id) => choice(id));
  const stop = paperStops(questions, [
    groupOf(["q0", "q1", "q2", "q3"], [["q0", "q1", "q2", "q3"].map(bound)]),
  ])[0]!;
  const picked = (id: string) => ({ type: "choice" as const, optionIds: [`${id}-a`] });

  it("opens the next gap with no answer, going round", () => {
    expect(nextEmptyGap(stop, 0, questions, { q0: picked("q0") })).toBe(1);
    expect(
      nextEmptyGap(stop, 2, questions, {
        q1: picked("q1"),
        q2: picked("q2"),
        q3: picked("q3"),
      }),
    ).toBe(0);
  });

  it("stays put when every other gap is answered", () => {
    const all = Object.fromEntries(questions.map((q) => [q.id, picked(q.id)]));
    expect(nextEmptyGap(stop, 1, questions, all)).toBeNull();
  });
});

describe("sharedPrompt", () => {
  const plainRich = (text: string) => ({
    format: "semantic_v1",
    blocks: [{ type: "paragraph", content: [{ type: "text", text, marks: [] }] }],
  });
  const runOf = (questions: StudentQuestion[]) =>
    paperStops(questions, [
      groupOf(
        questions.map((q) => q.id),
        [questions.map((q) => bound(q.id))],
      ),
    ])[0]!;

  it("is the prompt every member shares, plain or as one plain paragraph", () => {
    const questions = [
      choice("q0"),
      choice("q1", {
        promptContent: plainRich("Choose the best option for each gap.") as NonNullable<
          StudentQuestion["promptContent"]
        >,
      }),
    ];
    expect(sharedPrompt(runOf(questions), questions)?.id).toBe("q0");
  });

  it("is none when the members' prompts differ", () => {
    const questions = [choice("q0"), choice("q1", { prompt: "Which verb fits?" })];
    expect(sharedPrompt(runOf(questions), questions)).toBeNull();
  });

  it("is none when a member's prompt is rich", () => {
    const bold = {
      format: "semantic_v1",
      blocks: [
        {
          type: "paragraph",
          content: [
            {
              type: "text",
              text: "Choose the best option for each gap.",
              marks: ["bold"],
            },
          ],
        },
      ],
    } as unknown as NonNullable<StudentQuestion["promptContent"]>;
    const questions = [choice("q0"), choice("q1", { promptContent: bold })];
    expect(sharedPrompt(runOf(questions), questions)).toBeNull();
  });
});

describe("chosenWord", () => {
  const t = i18n.t.bind(i18n) as TFunction;

  it("is the chosen option's text, or null with nothing chosen", () => {
    const question = choice("q0");
    expect(chosenWord(question, { type: "choice", optionIds: ["q0-b"] }, t)).toBe(
      "relaxing",
    );
    expect(chosenWord(question, undefined, t)).toBeNull();
    expect(chosenWord(question, { type: "choice", optionIds: [] }, t)).toBeNull();
  });

  it("names a canonical true/false option as the engine does", async () => {
    await i18n.changeLanguage("vi");
    const question = choice("q0", {
      type: "true_false",
      options: [{ id: "t", text: "True" }],
    });
    expect(chosenWord(question, { type: "choice", optionIds: ["t"] }, t)).toBe("Đúng");
  });
});
