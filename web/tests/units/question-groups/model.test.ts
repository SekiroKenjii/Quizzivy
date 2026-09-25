import {
  groupIssue,
  emptyGroup,
  emptyMaterial,
  memberValues,
} from "@/features/question-groups/model";
import {
  independentBundle,
  readGroupRecovery,
} from "@/features/question-groups/recovery";
import type { GroupBundle } from "@/features/question-groups/api";

function group(): GroupBundle {
  const bundle = emptyGroup("Reading");
  const id = crypto.randomUUID();
  bundle.questions = [
    {
      id,
      input: {
        type: "single_choice",
        prompt: "Choose",
        points: 1,
        options: [
          { text: "A", isCorrect: true },
          { text: "B", isCorrect: false },
        ],
      },
    },
  ];
  bundle.group.members = [{ questionId: id, optionOrder: "fixed" }];
  const material = emptyMaterial("Passage");
  material.content = {
    format: "semantic_v1",
    blocks: [{ type: "paragraph", content: [{ type: "gap", id: "gap1", label: "1" }] }],
  };
  material.gaps = [{ kind: "question", gapId: "gap1", questionId: id }];
  bundle.group.stimuli = [material];
  return bundle;
}

test("bindings use identity and must cover every material gap exactly once", () => {
  const bundle = group();
  expect(groupIssue(bundle)).toBeNull();
  bundle.group.stimuli[0]!.gaps[0]!.gapId = "another-gap";
  expect(groupIssue(bundle)).toBe("groups.unboundGaps");
  bundle.group.stimuli[0]!.gaps = [];
  expect(groupIssue(bundle)).toBe("groups.unboundGaps");
});

test("one answer cannot be linked from two materials and detached members cannot save", () => {
  const bundle = group();
  const material = structuredClone(bundle.group.stimuli[0]!);
  material.id = crypto.randomUUID();
  bundle.group.stimuli.push(material);
  expect(groupIssue(bundle)).toBe("groups.duplicateTarget");
  bundle.group.members = [];
  expect(groupIssue(bundle)).toBe("groups.invalidQuestion");
});

test("recovery preserves incomplete user input without accepting a malformed graph", () => {
  const bundle = group();
  bundle.questions[0]!.input.prompt = "";
  const payload = { version: 1, revision: 4, bundle };
  expect(
    readGroupRecovery(payload, bundle.group.id)?.bundle.questions[0]!.input.prompt,
  ).toBe("");
  expect(readGroupRecovery(payload, crypto.randomUUID())).toBeNull();
  bundle.group.members.push(bundle.group.members[0]!);
  expect(readGroupRecovery(payload, bundle.group.id)).toBeNull();
  expect(
    readGroupRecovery({ ...payload, bundle: { group: null } }, bundle.group.id),
  ).toBeNull();
});

test("conflict recovery creates a fully independent graph with remapped links", () => {
  const original = group();
  const copied = independentBundle(original);
  expect(copied.group.id).not.toBe(original.group.id);
  expect(copied.group.stimuli[0]!.id).not.toBe(original.group.stimuli[0]!.id);
  expect(copied.group.stimuli[0]!.gaps[0]!.questionId).toBe(copied.questions[0]!.id);
  expect(copied.group.members[0]!.questionId).toBe(copied.questions[0]!.id);
  expect(groupIssue(copied)).toBeNull();
  copied.questions[0]!.input.prompt = "New";
  expect(original.questions[0]!.input.prompt).toBe("Choose");
});

test("material titles count Unicode code points and full editor inputs clear omitted rich content explicitly", () => {
  const bundle = group();
  bundle.group.title = "😀".repeat(200);
  expect(groupIssue(bundle)).toBeNull();
  const value = memberValues(bundle.questions[0]!.input);
  expect(value.promptContent).toBeNull();
  expect(value.options[0]!.content).toBeNull();
});

test("learner preview cannot receive answer keys, explanations or shared transcripts", async () => {
  const { groupPreview } = await import("@/features/question-groups/preview");
  const bundle = group();
  bundle.questions[0]!.input.sampleAnswer = "private answer";
  bundle.questions[0]!.input.explanation = "private explanation";
  bundle.questions[0]!.input.transcript = "private transcript";
  bundle.group.recordings = [
    {
      id: crypto.randomUUID(),
      assetId: crypto.randomUUID(),
      policy: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
      transcript: "private recording",
    },
  ];
  const projected = groupPreview(bundle, []);
  const json = JSON.stringify(projected);
  for (const forbidden of [
    "isCorrect",
    "sampleAnswer",
    "acceptedAnswers",
    "transcript",
    "explanation",
    "private",
  ])
    expect(json).not.toContain(forbidden);
  expect(projected.questions[0]!.id).toBe(projected.groups[0]!.questionIds[0]);
  expect(projected.groups[0]!.stimuli[0]!.gaps[0]!.questionId).toBe(
    projected.questions[0]!.id,
  );
});

test("independent recovery remaps both ends of rich blank links without mutating the original", () => {
  const bundle = group();
  const question = bundle.questions[0]!;
  bundle.group.members[0]!.optionOrder = "shuffle";
  question.input = {
    type: "fill_blank",
    points: 1,
    prompt: "[1]",
    promptContent: {
      format: "semantic_v1",
      blocks: [
        { type: "paragraph", content: [{ type: "gap", id: "answer-gap", label: "1" }] },
      ],
    },
    options: [],
    blanks: [
      {
        ordinal: 1,
        gapId: "answer-gap",
        acceptedAnswers: ["Monday"],
        caseSensitive: false,
      },
    ],
  };
  bundle.group.stimuli[0]!.gaps = [
    { kind: "blank", gapId: "gap1", questionId: question.id, blankGapId: "answer-gap" },
  ];
  const copied = independentBundle(bundle);
  expect(groupIssue(copied)).toBeNull();
  expect(copied.group.stimuli[0]!.gaps[0]!.gapId).not.toBe("gap1");
  expect(copied.group.stimuli[0]!.gaps[0]).toMatchObject({
    kind: "blank",
    questionId: copied.questions[0]!.id,
    blankGapId: copied.questions[0]!.input.blanks![0]!.gapId,
  });
  expect(copied.questions[0]!.input.blanks![0]!.gapId).not.toBe("answer-gap");
  expect(bundle.questions[0]!.input.blanks![0]!.gapId).toBe("answer-gap");
  expect(JSON.stringify(bundle.group.stimuli)).toContain("gap1");
});

test("oversized and cyclic local payloads are rejected before recovery rendering", () => {
  const bundle = group();
  const cyclic: { self?: unknown } = {};
  cyclic.self = cyclic;
  expect(readGroupRecovery(cyclic, bundle.group.id)).toBeNull();
  bundle.group.title = "a".repeat(4 * 1024 * 1024);
  expect(
    readGroupRecovery({ version: 1, revision: 1, bundle }, bundle.group.id),
  ).toBeNull();
});
