import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { previewRules } from "@/features/assignments/studentRules";
import i18n, { loadLocale } from "@/lib/i18n";

const DRAFT = {
  review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
  integrity: {
    requireFullscreen: true,
    blockCopyPaste: true,
    maxFocusLoss: 2,
    onLimitExceeded: "flag" as const,
    minAwayMs: 3000,
  },
  opensAt: "2026-08-29T08:00",
  closesAt: "2026-08-29T21:00",
};

function sentences(over: Partial<typeof DRAFT> = {}, locale: "vi" | "en" = "vi") {
  return previewRules(
    { ...DRAFT, ...over },
    i18n.getFixedT(locale),
    locale,
    new Date(),
  ).map((rule) => rule.text);
}

beforeEach(async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
  await loadLocale("en");
});
afterEach(() => {
  vi.useRealTimers();
});

describe("what the wizard tells the teacher students will read", () => {
  it("is the student's own list for the switches and dates of the draft", () => {
    expect(sentences()).toEqual([
      "Bài mở đến 21:00 hôm nay.",
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
      "Bài chạy ở chế độ toàn màn hình.",
      "Sao chép và dán bị tắt.",
      "Bạn được rời trang làm bài 2 lần. Sau đó, giáo viên sẽ được báo.",
      "Bạn sẽ xem được điểm sau khi nộp bài.",
    ]);
  });

  it("follows each switch as the teacher turns it", () => {
    expect(
      sentences({
        review: { showScore: true, showCorrectAnswers: true, showExplanations: true },
        integrity: {
          ...DRAFT.integrity,
          requireFullscreen: false,
          blockCopyPaste: false,
          maxFocusLoss: -1,
        },
      }),
    ).toEqual([
      "Bài mở đến 21:00 hôm nay.",
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
      "Không được rời trang làm bài. Nếu bạn rời trang, giáo viên sẽ được báo.",
      "Bạn sẽ xem được điểm, đáp án đúng và giải thích sau khi nộp bài.",
    ]);
  });

  it("reads the draft's dates in the app's zone, as a paper still to open", () => {
    expect(
      sentences({ opensAt: "2026-09-01T08:00", closesAt: "2026-09-03T21:00" })[0],
    ).toBe("Bài mở từ 08:00, Thứ 3, 01/09 đến 21:00, Thứ 5, 03/09.");
  });

  it.each([
    ["not filled in", "", "2026-08-29T21:00"],
    ["not filled in", "2026-08-29T08:00", ""],
    ["already past", "2026-08-27T08:00", "2026-08-29T16:59"],
    ["before the opening", "2026-09-03T08:00", "2026-09-01T21:00"],
    ["before an opening that has passed", "2026-08-28T08:00", "2026-08-27T21:00"],
  ])("leaves the dates out while the close is %s", (_, opensAt, closesAt) => {
    expect(sentences({ opensAt, closesAt })[0]).toBe(
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
    );
  });

  it("follows the teacher's language", () => {
    expect(sentences({}, "en")).toContain(
      "You can leave the test 2 times. After that, your teacher is told.",
    );
  });

  it("writes the dates in the teacher's language", () => {
    expect(
      sentences({ opensAt: "2026-09-01T08:00", closesAt: "2026-09-03T21:00" }, "en")[0],
    ).toBe("Opens Tue 1 Sep, 08:00 – Thu 3 Sep, 21:00.");
  });
});
