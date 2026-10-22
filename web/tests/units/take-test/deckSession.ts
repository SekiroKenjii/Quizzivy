import type { AttemptSession, StudentGroup } from "@/features/take-test/api";
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
        type: "tfng",
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
        selectCount: 2,
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
        maxWords: 3,
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
        maxWords: 2,
      },
      {
        id: id("b008"),
        sectionId: SECTION,
        type: "tfng",
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

const word = (text: string) => ({ type: "text" as const, text, marks: [] });

function paperOf(
  now: Date,
  minutes: number,
  over: Pick<AttemptSession, "testTitle" | "sections" | "questions" | "groups">,
  tail: string,
): AttemptSession {
  const deadlineAt = new Date(now.getTime() + minutes * 60_000).toISOString();
  return {
    attempt: {
      id: id(`0${tail}`),
      assignmentId: id(`d${tail}`),
      studentId: studentUser.id,
      testVersionId: id(`f${tail}`),
      attemptNo: 1,
      status: "in_progress",
      startedAt: now.toISOString(),
      deadlineAt,
      integrity: { focusLossCount: 0, flagged: false },
    },
    remainingAttempts: 1,
    sessionId: id(`5${tail}`),
    beaconToken: "deck-beacon-token",
    serverTime: now.toISOString(),
    audioPlays: {},
    groupAudioPlays: {},
    answers: {},
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
    ...over,
  };
}

export const DECK_MOCK_TITLE = "Grade 9 Mock Test 3";
export const DECK_MOCK_PROMPT =
  "Read the passage and choose the best option for each gap.";

const mock = {
  section: (n: number) => id(`9e0${n}`),
  question: (no: number) => id(`9a${String(no).padStart(2, "0")}`),
  options: (no: number, texts: readonly string[]) =>
    texts.map((text, index) => ({
      id: id(`9${String(no).padStart(2, "0")}${index + 1}`),
      text,
    })),
  blank: (no: number) => id(`9b${String(no).padStart(2, "0")}`),
};

const MOCK_SECTIONS = [
  [
    "Section 1 · Pronunciation",
    "Choose the word whose underlined part is pronounced differently, or the word with a different stress pattern.",
  ],
  [
    "Section 2 · Grammar and vocabulary",
    "Choose the best answer to complete each sentence.",
  ],
  [
    "Section 3 · Reading",
    "Read the notice and the passage, then answer the questions.",
  ],
  [
    "Section 4 · Writing",
    "Rewrite each sentence so that it means the same as the first one.",
  ],
] as const;

const MOCK_CHOICES: readonly (readonly [number, string, readonly string[]])[] = [
  [
    1,
    "Choose the word whose underlined part is pronounced differently.",
    ["cheap", "chair", "school", "church"],
  ],
  [
    1,
    "Choose the word whose underlined part is pronounced differently.",
    ["walked", "played", "wanted", "cooked"],
  ],
  [
    1,
    "Choose the word that has a different stress pattern.",
    ["happy", "begin", "open", "water"],
  ],
  [
    1,
    "Choose the word that has a different stress pattern.",
    ["decide", "teacher", "student", "doctor"],
  ],
  [2, "If I ___ you, I would study harder.", ["am", "was", "were", "be"]],
  [2, "She has lived here ___ 2019.", ["for", "since", "in", "at"]],
  [2, "My brother is interested ___ collecting stamps.", ["on", "at", "in", "with"]],
  [
    2,
    "The film was so ___ that we left early.",
    ["bored", "boring", "bore", "boredom"],
  ],
  [
    2,
    "They ___ dinner when the phone rang.",
    ["have", "had", "were having", "are having"],
  ],
  [2, "I suggest ___ to the museum on Sunday.", ["go", "going", "to go", "went"]],
  [
    2,
    "This is the house ___ my grandparents live.",
    ["which", "who", "where", "whose"],
  ],
  [
    2,
    "We couldn’t go out ___ the heavy rain.",
    ["because", "although", "because of", "despite"],
  ],
  [2, "He asked me where I ___.", ["live", "lived", "living", "am living"]],
  [2, "The letter ___ yesterday.", ["sent", "was sent", "is sent", "has sent"]],
  [
    2,
    "Lan is the ___ student in our class.",
    ["tall", "taller", "tallest", "most tall"],
  ],
  [2, "I’m looking forward ___ you soon.", ["see", "seeing", "to seeing", "to see"]],
  [
    2,
    "You ___ wear a uniform at our school. It’s a rule.",
    ["must", "might", "can", "would"],
  ],
  [2, "Nam ___ football every Sunday.", ["play", "plays", "playing", "is play"]],
  [2, "Hoa, ___ is my best friend, lives next door.", ["that", "which", "who", "whom"]],
  [
    2,
    "It’s the first time I ___ this dish.",
    ["try", "tried", "have tried", "am trying"],
  ],
  [
    3,
    "LIBRARY: Closed on Saturday afternoons. What does the notice say?",
    [
      "The library is closed all Saturday.",
      "You can visit on Saturday morning.",
      "The library opens only on Saturdays.",
      "Books must be returned on Saturday.",
    ],
  ],
  [
    3,
    "NO CYCLING IN THE PARK. Where would you see this sign?",
    ["At a park gate", "In a classroom", "On a bus", "In a shop"],
  ],
];

const CLOZE_OPTIONS = [
  ["relax", "relaxing", "relaxed", "relaxes"],
  ["do", "make", "take", "get"],
  ["cover", "covering", "covered", "covers"],
  ["any", "some", "much", "every"],
  ["do", "get", "make", "have"],
] as const;

const CLOZE_TEXT = [
  "Green spaces matter in big cities. Parks give people a place to ",
  " after work. Trees also ",
  " the air cleaner. Many cities now ",
  " roofs with plants, and ",
  " streets are closed to cars on Sundays. Experts believe these changes will ",
  " city life healthier.",
] as const;

const STATEMENTS = [
  "Parks give people somewhere to relax after work.",
  "Every street in the city is closed to cars on Sundays.",
  "Most people in the city own a bicycle.",
] as const;

const REWRITES = [
  "He started learning English five years ago. → He has {{1}}",
  "“I am tired,” she said. → She said that {{1}}",
  "It’s a pity I can’t swim. → I wish {{1}}",
  "They built this bridge in 1990. → This bridge {{1}}",
] as const;

/**
 * deckMockTestSession is the deck's paper s9, "Grade 9 Mock Test 3", without
 * its matching question (R6): 34 questions in four sections. The cloze is a
 * group in section 3 with one material holding five gaps, each bound to a
 * single-choice member (23 to 27), followed by the three statements as
 * three-option single choice (28 to 30). It carries no answer key.
 */
export function deckMockTestSession(now: Date): AttemptSession {
  const choices = MOCK_CHOICES.map(([section, prompt, texts], at) => ({
    id: mock.question(at + 1),
    sectionId: mock.section(section),
    type: "single_choice" as const,
    prompt,
    points: 1,
    options: mock.options(at + 1, texts),
  }));
  const cloze = CLOZE_OPTIONS.map((texts, at) => ({
    id: mock.question(23 + at),
    sectionId: mock.section(3),
    type: "single_choice" as const,
    prompt: DECK_MOCK_PROMPT,
    points: 1,
    options: mock.options(23 + at, texts),
  }));
  const statements = STATEMENTS.map((prompt, at) => ({
    id: mock.question(28 + at),
    sectionId: mock.section(3),
    type: "single_choice" as const,
    prompt,
    points: 1,
    options: mock.options(28 + at, ["True", "False", "Not given"]),
  }));
  const rewrites = REWRITES.map((prompt, at) => ({
    id: mock.question(31 + at),
    sectionId: mock.section(4),
    type: "fill_blank" as const,
    prompt,
    points: 1,
    blanks: [{ id: mock.blank(31 + at), ordinal: 1, caseSensitive: false }],
  }));
  const gapId = (at: number) => `gap-${23 + at}`;
  const reading: StudentGroup = {
    id: id("9f01"),
    sectionId: mock.section(3),
    title: "Reading · Green spaces",
    questionIds: [...cloze, ...statements].map((question) => question.id),
    stimuli: [
      {
        id: id("9f02"),
        title: "Green spaces in big cities",
        content: {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: CLOZE_TEXT.flatMap((text, at) =>
                at < cloze.length
                  ? [
                      word(text),
                      { type: "gap" as const, id: gapId(at), label: String(23 + at) },
                    ]
                  : [word(text)],
              ),
            },
          ],
        },
        gaps: cloze.map((question, at) => ({
          kind: "question" as const,
          gapId: gapId(at),
          questionId: question.id,
        })),
      },
    ],
    recordings: [],
    assets: [],
  };
  return paperOf(
    now,
    60,
    {
      testTitle: DECK_MOCK_TITLE,
      sections: MOCK_SECTIONS.map(([title, instructions], at) => ({
        id: mock.section(at + 1),
        title,
        instructions,
      })),
      questions: [...choices, ...cloze, ...statements, ...rewrites],
      groups: [reading],
    },
    "0e9",
  );
}

export const DECK_LISTENING_TITLE = "Unit 4 · Listening: Directions";

const listening = {
  section: (n: number) => id(`2e0${n}`),
  question: (no: number) => id(`2a${String(no).padStart(2, "0")}`),
  options: (no: number, texts: readonly string[]) =>
    texts.map((text, index) => ({
      id: id(`2${String(no).padStart(2, "0")}${index + 1}`),
      text,
    })),
};

const MAP_PLACES = ["Post office", "Bank", "Pharmacy", "Bus stop", "Bookshop"] as const;

const WALK = [
  [
    "The walk starts at the…",
    ["old market", "river bridge", "town hall", "train station"],
  ],
  [
    "How long does the walk take?",
    ["45 minutes", "an hour", "an hour and a half", "two hours"],
  ],
  [
    "What should visitors bring?",
    ["an umbrella", "a camera", "comfortable shoes", "a map"],
  ],
  [
    "Where does the walk finish?",
    ["at a café", "in the park", "at the museum", "by the river"],
  ],
] as const;

function listeningGroup(
  part: 1 | 2,
  title: string,
  material: string,
  seconds: number,
  allowPause: boolean,
  questionIds: string[],
): StudentGroup {
  const assetId = id(`2f${part}4`);
  return {
    id: id(`2f${part}1`),
    sectionId: listening.section(part),
    title,
    questionIds,
    stimuli: [
      {
        id: id(`2f${part}2`),
        title: material,
        content: {
          format: "semantic_v1",
          blocks: [{ type: "audio", assetId, label: material }],
        },
        gaps: [],
      },
    ],
    recordings: [
      {
        id: id(`2f${part}3`),
        assetId,
        policy: {
          maxPlays: 2,
          allowSeek: false,
          showTranscriptAfterSubmit: false,
          allowPause,
        },
      },
    ],
    assets: [
      {
        id: assetId,
        kind: "audio" as const,
        mimeType: "audio/mpeg",
        bytes: seconds * 16_000,
        durationMs: seconds * 1000,
        originalFilename: `${material.toLowerCase().replaceAll(" ", "-")}.mp3`,
        createdAt: "2026-09-20T00:00:00.000Z",
        url: `https://assets.example/deck/part-${part}.mp3`,
      },
    ],
  };
}

/**
 * deckListeningSession is the deck's paper s2, "Unit 4 · Listening:
 * Directions": two listening groups, each with one material that holds only
 * its audio node and one recording of two plays; part 2's cannot be paused,
 * as the deck draws it (`canPause: false`). Part 1's map labelling, a
 * matching question in the deck (R6), stands as five single-choice questions
 * so part 2 keeps the deck's numbers, 6 to 10. It carries no answer key.
 */
export function deckListeningSession(now: Date): AttemptSession {
  const letters = ["A", "B", "C", "D", "E", "F"];
  const map = MAP_PLACES.map((place, at) => ({
    id: listening.question(at + 1),
    sectionId: listening.section(1),
    type: "single_choice" as const,
    prompt: `Label the map. Choose the letter (A–F) for the ${place.toLowerCase()}.`,
    points: 1,
    options: listening.options(at + 1, letters),
  }));
  const walk = WALK.map(([prompt, texts], at) => ({
    id: listening.question(6 + at),
    sectionId: listening.section(2),
    type: "single_choice" as const,
    prompt,
    points: 1,
    options: listening.options(6 + at, texts),
  }));
  const street = {
    id: listening.question(10),
    sectionId: listening.section(2),
    type: "short_answer" as const,
    prompt: "What is the name of the street with the oldest houses?",
    points: 1,
  };
  return paperOf(
    now,
    30,
    {
      testTitle: DECK_LISTENING_TITLE,
      sections: [
        {
          id: listening.section(1),
          title: "Part 1 · Asking the way",
          instructions: "Listen to Minh asking for directions, then label the map.",
        },
        {
          id: listening.section(2),
          title: "Part 2 · A guided walk",
          instructions:
            "Listen to a tour guide talking about a walk in the old town. Answer questions 6–10.",
        },
      ],
      questions: [...map, ...walk, street],
      groups: [
        listeningGroup(
          1,
          "Part 1 · Asking the way",
          "Asking the way",
          154,
          true,
          map.map((question) => question.id),
        ),
        listeningGroup(
          2,
          "Part 2 · A guided walk",
          "A guided walk",
          212,
          false,
          [...walk, street].map((question) => question.id),
        ),
      ],
    },
    "0e2",
  );
}
