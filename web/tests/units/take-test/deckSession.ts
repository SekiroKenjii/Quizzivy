import type { AttemptSession } from "@/features/take-test/api";
import { studentUser } from "@tests/support/fixtures";

export const DECK_ATTEMPT = "018f0000-0000-7000-8000-0000000000e1";
export const DECK_TITLE = "Reading · Why cities need green space";
export const DECK_LEFT_MS = 38 * 60_000 + 12_000;

const id = (tail: string) => `018f0000-0000-7000-8000-00000000${tail}`;
const SECTION = id("a001");

const choice = (question: number, texts: string[]) =>
  texts.map((text, index) => ({ id: id(`c${question}0${index + 1}`), text }));

export function deckSession(now: Date, leftMs = DECK_LEFT_MS): AttemptSession {
  return {
    attempt: {
      id: DECK_ATTEMPT,
      assignmentId: id("00d1"),
      studentId: studentUser.id,
      testVersionId: id("00f1"),
      attemptNo: 1,
      status: "in_progress",
      startedAt: new Date(now.getTime() + leftMs - 45 * 60_000).toISOString(),
      deadlineAt: new Date(now.getTime() + leftMs).toISOString(),
      integrity: { focusLossCount: 0, flagged: false },
    },
    remainingAttempts: 0,
    testTitle: DECK_TITLE,
    sections: [{ id: SECTION, title: "Reading passage 1", instructions: null }],
    questions: [
      {
        id: id("b001"),
        sectionId: SECTION,
        type: "single_choice",
        prompt: "What is the main purpose of the passage?",
        points: 1,
        options: choice(1, [
          "To compare parks in European and Asian cities",
          "To argue that green space is worth planning for",
          "To describe the history of city parks",
          "To criticise councils that close streets",
        ]),
      },
      {
        id: id("b002"),
        sectionId: SECTION,
        type: "single_choice",
        prompt: "According to paragraph A, people who live near a park…",
        points: 1,
        options: choice(2, [
          "move house less often",
          "sleep better and see the doctor less",
          "spend more time outside at night",
          "pay less for their homes",
        ]),
      },
      {
        id: id("b003"),
        sectionId: SECTION,
        type: "single_choice",
        prompt: "Homes near parks always sell for 15 per cent more.",
        points: 1,
        options: choice(3, ["True", "False", "Not given"]),
      },
      {
        id: id("b004"),
        sectionId: SECTION,
        type: "multiple_choice",
        prompt: "Which TWO benefits of trees and parks does paragraph B mention?",
        points: 1,
        options: choice(4, [
          "Lower street temperatures",
          "Less traffic noise",
          "Higher property values",
          "Cleaner rivers",
          "More tourists",
        ]),
      },
      {
        id: id("b005"),
        sectionId: SECTION,
        type: "short_answer",
        prompt: "What should cities prioritise when they plan new districts?",
        points: 1,
      },
      {
        id: id("b006"),
        sectionId: SECTION,
        type: "single_choice",
        prompt: "What rule does Copenhagen follow?",
        points: 1,
        options: choice(6, [
          "Parks must be larger than five hectares",
          "Every home must be a five-minute walk from a park",
          "New districts must have no cars",
          "Trees must line every street",
        ]),
      },
      {
        id: id("b007"),
        sectionId: SECTION,
        type: "short_answer",
        prompt: "What do locals call the streets closed to traffic on Sundays?",
        points: 1,
      },
      {
        id: id("b008"),
        sectionId: SECTION,
        type: "single_choice",
        prompt: "Shop owners on play streets were against the idea at first.",
        points: 1,
        options: choice(8, ["True", "False", "Not given"]),
      },
    ],
    sessionId: id("5e55"),
    beaconToken: "deck-beacon-token",
    serverTime: now.toISOString(),
    audioPlays: {},
    answers: {
      [id("b001")]: { type: "choice", optionIds: [id("c102")] },
      [id("b002")]: { type: "choice", optionIds: [id("c202")] },
      [id("b003")]: { type: "choice", optionIds: [id("c302")] },
    },
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
  };
}

export function deckSaved(now: Date, deadlineAt: string) {
  return { serverTime: now.toISOString(), savedAt: now.toISOString(), deadlineAt };
}

export const DECK_PASSAGE = "Why cities need green space";

const PARAGRAPHS = [
  [
    "A",
    "Planners once treated parks as a luxury, something to add only after housing and roads were finished. That view has changed. Research across forty European cities found that residents who lived within 300 metres of a park reported better sleep and visited their doctor less often.",
  ],
  [
    "B",
    "Green space also pays for itself. Homes that overlook a park sell for up to 15 per cent more than similar homes a few streets away, and the extra property tax often covers the cost of maintenance within a decade. Trees along busy streets cool the pavement by several degrees in summer.",
  ],
  [
    "C",
    "For this reason, the report argues that cities should prioritise public green space when they plan new districts, rather than leaving small leftover plots between buildings. It points to Copenhagen, where every new neighbourhood must place a park within a five-minute walk of each home.",
  ],
  [
    "D",
    "Some councils have gone further. On Sundays, a number of streets in Bogotá and Seoul are closed to traffic and handed to walkers and cyclists. Locals call them play streets, and surveys show that most shop owners on these streets now support the scheme they once opposed.",
  ],
] as const;

export function deckPassageSession(now: Date, leftMs = DECK_LEFT_MS): AttemptSession {
  const paper = deckSession(now, leftMs);
  return {
    ...paper,
    groups: [
      {
        id: id("9001"),
        sectionId: SECTION,
        title: DECK_PASSAGE,
        questionIds: paper.questions.map((question) => question.id),
        stimuli: [
          {
            id: id("9002"),
            title: DECK_PASSAGE,
            content: {
              format: "semantic_v1",
              blocks: PARAGRAPHS.map(([key, text]) => ({
                type: "paragraph",
                content: [
                  { type: "text", text: key, marks: ["bold"] },
                  { type: "text", text: `\u2002${text}`, marks: [] },
                ],
              })),
            },
            gaps: [],
          },
        ],
        recordings: [],
        assets: [],
      },
    ],
    groupAudioPlays: {},
  };
}
