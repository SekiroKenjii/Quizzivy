import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { StudentRulesPreview } from "@/features/assignments/components/StudentRulesPreview";
import i18n from "@/lib/i18n";

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

function sentences(over: Partial<typeof DRAFT> = {}) {
  render(<StudentRulesPreview draft={{ ...DRAFT, ...over }} />);
  return screen
    .getByText("Học viên sẽ đọc")
    .nextElementSibling!.textContent!.split("· ")
    .slice(1);
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-08-29T10:00:00Z"));
});
afterEach(async () => {
  vi.useRealTimers();
  await i18n.changeLanguage("vi");
});

describe("what the teacher is told students will read", () => {
  it("is the student's own list for the switches and dates on the form", () => {
    expect(sentences()).toEqual([
      "Bài mở đến 21:00 hôm nay.",
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
      "Bài chạy ở chế độ toàn màn hình.",
      "Sao chép và dán bị tắt.",
      "Bạn được rời trang làm bài 2 lần. Sau đó, giáo viên sẽ được báo.",
      "Bạn sẽ xem được điểm sau khi nộp bài.",
    ]);
  });

  it("reads the form's dates in the app's zone, as a paper still to open", () => {
    expect(
      sentences({ opensAt: "2026-09-01T08:00", closesAt: "2026-09-03T21:00" })[0],
    ).toBe("Bài mở từ 08:00, Thứ 3, 01/09 đến 21:00, Thứ 5, 03/09.");
  });

  it.each([
    ["", "2026-08-29T21:00"],
    ["2026-08-29T08:00", ""],
  ])("leaves the dates out while one is not filled in", (opensAt, closesAt) => {
    expect(sentences({ opensAt, closesAt })[0]).toBe(
      "Đồng hồ chạy từ lúc bạn bấm Bắt đầu và không dừng lại, kể cả khi bạn đóng trang.",
    );
  });

  it("follows the form's language", async () => {
    await i18n.changeLanguage("en");
    render(<StudentRulesPreview draft={DRAFT} />);
    expect(
      screen.getByText(
        "· You can leave the test 2 times. After that, your teacher is told.",
      ),
    ).toBeInTheDocument();
  });
});
