import { describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import type { IntegrityPolicy, ReviewPolicy } from "@/features/assignments/api";
import { studentRules, type RulesInput } from "@/features/assignments/studentRules";

const NOW = new Date("2026-08-29T10:00:00Z");
const LANGUAGES = ["vi", "en"] as const;
type Language = (typeof LANGUAGES)[number];
type Action = IntegrityPolicy["onLimitExceeded"];

const REVIEW: ReviewPolicy = {
  showScore: true,
  showCorrectAnswers: false,
  showExplanations: false,
};
const INTEGRITY: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 0,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};
const WINDOW = {
  opensAt: "2026-08-29T01:00:00Z",
  closesAt: "2026-08-29T14:00:00Z",
  upcoming: false,
};

function rules(over: Partial<RulesInput> = {}, language: Language = "vi") {
  return studentRules(
    { review: REVIEW, integrity: INTEGRITY, window: WINDOW, ...over },
    i18n.getFixedT(language),
    language,
    NOW,
  );
}

function texts(over: Partial<RulesInput> = {}, language: Language = "vi") {
  return rules(over, language).map((rule) => rule.text);
}

function only(kind: string, over: Partial<RulesInput>, language: Language = "vi") {
  return rules(over, language)
    .filter((rule) => rule.kind === kind)
    .map((rule) => rule.text);
}

const RECORDED = {
  vi: "Mỗi lần rời trang làm bài đều được ghi lại.",
  en: "Leaving the test is recorded.",
};

const LEAVING: readonly [number, Action, string, string][] = [
  [0, "flag", RECORDED.vi, RECORDED.en],
  [0, "warn", RECORDED.vi, RECORDED.en],
  [0, "auto_submit", RECORDED.vi, RECORDED.en],
  [
    -1,
    "flag",
    "Không được rời trang làm bài. Nếu bạn rời trang, giáo viên sẽ được báo.",
    "Leaving the test is not allowed. Your teacher will be told.",
  ],
  [
    -1,
    "warn",
    "Không được rời trang làm bài. Nếu bạn rời trang, bạn sẽ được nhắc.",
    "Leaving the test is not allowed. You will get a warning.",
  ],
  [
    -1,
    "auto_submit",
    "Không được rời trang làm bài. Nếu bạn rời trang, bài sẽ được nộp ngay.",
    "Leaving the test is not allowed. Your test will be submitted.",
  ],
  [
    1,
    "flag",
    "Bạn được rời trang làm bài 1 lần. Sau đó, giáo viên sẽ được báo.",
    "You can leave the test 1 time. After that, your teacher is told.",
  ],
  [
    2,
    "flag",
    "Bạn được rời trang làm bài 2 lần. Sau đó, giáo viên sẽ được báo.",
    "You can leave the test 2 times. After that, your teacher is told.",
  ],
  [
    1,
    "warn",
    "Bạn được rời trang làm bài 1 lần. Sau đó, bạn sẽ được nhắc.",
    "You can leave the test 1 time. After that, you will get a warning.",
  ],
  [
    2,
    "warn",
    "Bạn được rời trang làm bài 2 lần. Sau đó, bạn sẽ được nhắc.",
    "You can leave the test 2 times. After that, you will get a warning.",
  ],
  [
    1,
    "auto_submit",
    "Bạn được rời trang làm bài 1 lần. Sau đó, bài sẽ được nộp ngay.",
    "You can leave the test 1 time. After that, your test is submitted.",
  ],
  [
    2,
    "auto_submit",
    "Bạn được rời trang làm bài 2 lần. Sau đó, bài sẽ được nộp ngay.",
    "You can leave the test 2 times. After that, your test is submitted.",
  ],
];

const SCORE: readonly [boolean, boolean, string, string][] = [
  [
    false,
    false,
    "Bạn sẽ xem được điểm sau khi nộp bài.",
    "You will see your score after submitting.",
  ],
  [
    true,
    false,
    "Bạn sẽ xem được điểm và đáp án đúng sau khi nộp bài.",
    "You will see your score and the correct answers after submitting.",
  ],
  [
    false,
    true,
    "Bạn sẽ xem được điểm và giải thích sau khi nộp bài.",
    "You will see your score and explanations after submitting.",
  ],
  [
    true,
    true,
    "Bạn sẽ xem được điểm, đáp án đúng và giải thích sau khi nộp bài.",
    "You will see your score, the correct answers and explanations after submitting.",
  ],
];

const FULLSCREEN = {
  vi: "Bài chạy ở chế độ toàn màn hình.",
  en: "The test runs in fullscreen.",
};
const COPY = { vi: "Sao chép và dán bị tắt.", en: "Copy and paste are turned off." };
const ORDER = [
  "availability",
  "timer",
  "fullscreen",
  "copy",
  "leaving",
  "audio",
  "score",
];

describe("the default paper", () => {
  it("reads five sentences, in order, in Vietnamese", () => {
    expect(texts()).toEqual([
      "Bài mở đến 21:00 hôm nay.",
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
      COPY.vi,
      RECORDED.vi,
      "Bạn sẽ xem được điểm sau khi nộp bài.",
    ]);
  });

  it("reads the deck's sentences in English", () => {
    expect(texts({}, "en")).toEqual([
      "Available until today 21:00.",
      "The timer starts when you press Start and does not pause, even if you close the page.",
      COPY.en,
      RECORDED.en,
      "You will see your score after submitting.",
    ]);
  });
});

describe("leaving the test", () => {
  it.each(LEAVING)(
    "with a limit of %d and %s",
    (maxFocusLoss, onLimitExceeded, vi, en) => {
      const integrity = { ...INTEGRITY, maxFocusLoss, onLimitExceeded };
      expect(only("leaving", { integrity })).toEqual([vi]);
      expect(only("leaving", { integrity }, "en")).toEqual([en]);
    },
  );

  it("never names the teacher when the server only warns", () => {
    for (const [, action, vi, en] of LEAVING.filter(([max]) => max !== 0)) {
      if (action !== "warn") continue;
      expect(vi).not.toMatch(/giáo viên/);
      expect(en).not.toMatch(/teacher/);
    }
  });

  it("names no number when leaving is not allowed at all", () => {
    for (const [max, , vi, en] of LEAVING) {
      if (max >= 0) continue;
      expect(vi).not.toMatch(/\d/);
      expect(en).not.toMatch(/\d/);
    }
  });
});

const TOGGLES = [false, true].flatMap((requireFullscreen) =>
  [false, true].flatMap((blockCopyPaste) =>
    [false, true].map((showScore) => ({
      requireFullscreen,
      blockCopyPaste,
      showScore,
    })),
  ),
);

interface Toggles {
  readonly requireFullscreen: boolean;
  readonly blockCopyPaste: boolean;
  readonly showScore: boolean;
}

function expectedFor(
  language: Language,
  toggles: Toggles,
  leaving: string,
  score: string,
) {
  return {
    leaving: [leaving],
    fullscreen: toggles.requireFullscreen ? [FULLSCREEN[language]] : [],
    copy: toggles.blockCopyPaste ? [COPY[language]] : [],
    score: toggles.showScore ? [score] : [],
  };
}

function combinations() {
  return LEAVING.flatMap(([maxFocusLoss, onLimitExceeded, leaveVi, leaveEn]) =>
    TOGGLES.flatMap((toggles) =>
      SCORE.map(([showCorrectAnswers, showExplanations, scoreVi, scoreEn]) => ({
        input: {
          integrity: {
            ...INTEGRITY,
            maxFocusLoss,
            onLimitExceeded,
            requireFullscreen: toggles.requireFullscreen,
            blockCopyPaste: toggles.blockCopyPaste,
          },
          review: {
            showScore: toggles.showScore,
            showCorrectAnswers,
            showExplanations,
          },
        },
        expected: {
          vi: expectedFor("vi", toggles, leaveVi, scoreVi),
          en: expectedFor("en", toggles, leaveEn, scoreEn),
        },
      })),
    ),
  );
}

describe("every combination of the policies", () => {
  const cases = combinations();

  it("says each sentence exactly when its policy asks for it", () => {
    expect(cases).toHaveLength(384);
    for (const { input, expected } of cases) {
      for (const language of LANGUAGES) {
        for (const kind of ["leaving", "fullscreen", "copy", "score"] as const) {
          expect(only(kind, input, language)).toEqual(expected[language][kind]);
        }
      }
    }
  });

  it("keeps one order and gives every sentence its own id", () => {
    for (const { input } of cases) {
      const list = rules(input);
      const kinds = list.map((rule) => ORDER.indexOf(rule.kind));
      expect(kinds).toEqual([...kinds].sort((a, b) => a - b));
      expect(new Set(list.map((rule) => rule.id)).size).toBe(list.length);
    }
  });

  it("never speaks of a violation or of cheating", () => {
    for (const { input } of cases) {
      for (const language of LANGUAGES) {
        expect(texts(input, language).join(" ")).not.toMatch(
          /vi phạm|gian lận|violation|cheat/i,
        );
      }
    }
  });
});

describe("when the test is available", () => {
  it("names the day when it does not close today", () => {
    const window = { ...WINDOW, closesAt: "2026-09-24T01:00:00Z" };
    expect(only("availability", { window })).toEqual([
      "Bài mở đến 08:00, Thứ 5, 24/09.",
    ]);
    expect(only("availability", { window }, "en")).toEqual([
      "Available until Thu 24 Sep, 08:00.",
    ]);
  });

  it("says today of a window that opened on an earlier day and closes today", () => {
    const window = { ...WINDOW, opensAt: "2026-08-27T01:00:00Z" };
    expect(only("availability", { window })).toEqual(["Bài mở đến 21:00 hôm nay."]);
  });

  it("gives the opening and the close of a test that has not opened", () => {
    const window = {
      opensAt: "2026-09-28T01:00:00Z",
      closesAt: "2026-09-30T14:00:00Z",
      upcoming: true,
    };
    expect(only("availability", { window })).toEqual([
      "Bài mở từ 08:00, Thứ 2, 28/09 đến 21:00, Thứ 4, 30/09.",
    ]);
    expect(only("availability", { window }, "en")).toEqual([
      "Opens Mon 28 Sep, 08:00 – Wed 30 Sep, 21:00.",
    ]);
  });

  it("names the day once when it opens and closes on the same day", () => {
    const window = {
      opensAt: "2026-09-28T01:00:00Z",
      closesAt: "2026-09-28T14:00:00Z",
      upcoming: true,
    };
    expect(only("availability", { window })).toEqual([
      "Bài mở Thứ 2, 28/09, từ 08:00 đến 21:00.",
    ]);
    expect(only("availability", { window }, "en")).toEqual([
      "Opens Mon 28 Sep, 08:00 – 21:00.",
    ]);
  });

  it("says today when it opens and closes later today", () => {
    const window = {
      opensAt: "2026-08-29T12:00:00Z",
      closesAt: "2026-08-29T14:00:00Z",
      upcoming: true,
    };
    expect(only("availability", { window })).toEqual([
      "Bài mở hôm nay, từ 19:00 đến 21:00.",
    ]);
    expect(only("availability", { window }, "en")).toEqual([
      "Opens today, 19:00 – 21:00.",
    ]);
  });

  it("names the days when it opens today and closes on another day", () => {
    const window = {
      opensAt: "2026-08-29T12:00:00Z",
      closesAt: "2026-08-31T14:00:00Z",
      upcoming: true,
    };
    expect(only("availability", { window })).toEqual([
      "Bài mở từ 19:00, Thứ 7, 29/08 đến 21:00, Thứ 2, 31/08.",
    ]);
  });

  it("says nothing of dates when it is given none", () => {
    const list = studentRules(
      { review: REVIEW, integrity: INTEGRITY },
      i18n.getFixedT("vi"),
      "vi",
      NOW,
    );
    expect(list.map((rule) => rule.kind)).toEqual([
      "timer",
      "copy",
      "leaving",
      "score",
    ]);
  });
});

describe("the timer, fullscreen and audio", () => {
  it("says the timer is running for an attempt already begun", () => {
    expect(only("timer", { running: true })).toEqual([
      "Đồng hồ đang chạy và không dừng lại, kể cả khi bạn đóng trang.",
    ]);
    expect(only("timer", { running: true }, "en")).toEqual([
      "The timer is running and does not pause, even if you close the page.",
    ]);
  });

  it("does not promise fullscreen on a browser that has none", () => {
    const integrity = { ...INTEGRITY, requireFullscreen: true };
    expect(only("fullscreen", { integrity, fullscreenSupported: false })).toEqual([
      "Bài yêu cầu toàn màn hình, nhưng trình duyệt này không hỗ trợ. Bạn vẫn làm bài bình thường.",
    ]);
    expect(only("fullscreen", { integrity, fullscreenSupported: false }, "en")).toEqual(
      [
        "This test asks for fullscreen, but this browser has none. You can take it as normal.",
      ],
    );
    expect(only("fullscreen", { fullscreenSupported: false })).toEqual([]);
  });

  it("states the play limit, its singular, and unlimited replays", () => {
    expect(only("audio", { audio: { maxPlays: 2, shared: false } })).toEqual([
      "Mỗi bài nghe có giới hạn riêng, từ 2 lượt. Lượt nghe thêm được ghi lại.",
    ]);
    expect(only("audio", { audio: { maxPlays: 1, shared: false } }, "en")).toEqual([
      "Each recording has its own limit, starting at 1 play. Additional plays are recorded.",
    ]);
    expect(only("audio", { audio: { maxPlays: 2, shared: false } }, "en")).toEqual([
      "Each recording has its own limit, starting at 2 plays. Additional plays are recorded.",
    ]);
    expect(only("audio", { audio: { maxPlays: null, shared: false } })).toEqual([
      "Bài có câu nghe; bạn nghe lại được không giới hạn.",
    ]);
    expect(only("audio", {})).toEqual([]);
  });

  it("puts the audio sentences between leaving and the score", () => {
    const integrity = { ...INTEGRITY, requireFullscreen: true };
    const list = rules({ integrity, audio: { maxPlays: 2, shared: true } });
    expect(list.map((rule) => rule.id)).toEqual([
      "availability",
      "timer",
      "fullscreen",
      "copy",
      "leaving",
      "audio",
      "audio-shared",
      "score",
    ]);
  });

  it("adds the shared-recording sentence under its own id", () => {
    const list = rules({ audio: { maxPlays: 2, shared: true } });
    expect(list.filter((rule) => rule.kind === "audio").map((rule) => rule.id)).toEqual(
      ["audio", "audio-shared"],
    );
    expect(list.find((rule) => rule.id === "audio-shared")?.text).toBe(
      "Các câu dùng chung bài nghe cũng dùng chung lượt nghe. Đổi câu hoặc tải lại trang không đặt lại lượt nghe.",
    );
  });
});
