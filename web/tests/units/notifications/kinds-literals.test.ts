import { describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { describeNotification } from "@/features/notifications/kinds";
import type { NotificationParams } from "@/features/notifications/api";
import { listResponse, notices } from "./support";

const literals = [
  "Literal {{count}} Name",
  "{{className}} {{studentName}} {{title}} {{when}} {{names}}",
  "$& $1 $` $' {{nested {{count}}}} $t(notifications.kind.joined)",
  "<img src=x onerror=alert(1)> & Nguyễn Thị Thương 🧪",
  "\uE000quizzivy-notification:0\uE001 / \uE000quizzivy-notification:1\uE001",
] as const;

function text(index: number, params: NotificationParams, locale: "vi" | "en") {
  const notice = { ...notices[index]!, params };
  expect(() => listResponse({ items: [notice], nextBefore: null })).not.toThrow();
  return describeNotification(notice, i18n.getFixedT(locale), locale)?.text;
}

describe.each(["vi", "en"] as const)("notification literal strings in %s", (locale) => {
  it("preserves the exact flagged name before the actual count three", () => {
    expect(text(1, { studentName: literals[0], focusLost: 3 }, locale)).toBe(
      locale === "vi"
        ? "Literal {{count}} Name đã rời trang làm bài 3 lần"
        : "Literal {{count}} Name left the test 3 times",
    );
  });
  it.each([0, 1, 3])("keeps numeric plural selection at count %s", (count) => {
    const times = count === 1 ? "time" : "times";
    const students = count === 1 ? "student has" : "students have";
    const papers = count === 1 ? "paper" : "papers";
    expect(text(1, { studentName: literals[0], focusLost: count }, locale)).toBe(
      locale === "vi"
        ? `${literals[0]} đã rời trang làm bài ${count} lần`
        : `${literals[0]} left the test ${count} ${times}`,
    );
    expect(text(2, { title: literals[0], notSubmitted: count }, locale)).toBe(
      locale === "vi"
        ? `${literals[0]} sắp đóng, còn ${count} học viên chưa nộp`
        : `${literals[0]} closes soon and ${count} ${students} not handed in`,
    );
    expect(text(0, { title: literals[0], count, toGrade: 0 }, locale)).toBe(
      locale === "vi"
        ? `${count} bài đã nộp cho ${literals[0]}`
        : `${count} ${papers} handed in for ${literals[0]}`,
    );
  });
  it.each(literals)("preserves every string field literally: %s", (value) => {
    expect(text(1, { studentName: value, focusLost: 3 }, locale)).toBe(
      locale === "vi"
        ? `${value} đã rời trang làm bài 3 lần`
        : `${value} left the test 3 times`,
    );
    expect(text(3, { studentName: value, className: value }, locale)).toBe(
      locale === "vi" ? `${value} đã vào lớp ${value}` : `${value} joined ${value}`,
    );
    expect(text(4, { count: 3, classNames: [value, value] }, locale)).toBe(
      locale === "vi"
        ? `Mã tham gia của 3 lớp đã được đổi mới: ${value}, ${value}…`
        : `The join codes of 3 classes were replaced: ${value}, ${value}…`,
    );
    expect(text(0, { title: value, count: 3, toGrade: 1 }, locale)).toBe(
      locale === "vi"
        ? `1 bài nộp của ${value} đang chờ chấm`
        : `1 paper in ${value} needs grading`,
    );
    expect(text(8, { title: value }, locale)).toBe(
      locale === "vi" ? `${value} đã có kết quả` : `Your result for ${value} is ready`,
    );
  });
  it("preserves studentName before a different className", () => {
    const studentName = "Nam {{className}} $&";
    const className = "Lớp {{studentName}} $1";
    expect(text(3, { studentName, className }, locale)).toBe(
      locale === "vi"
        ? `${studentName} đã vào lớp ${className}`
        : `${studentName} joined ${className}`,
    );
  });
  it("preserves a title before the app-zone time in every timed kind", () => {
    const title = "Title {{when}} {{title}} $&";
    const params = { title, closesAt: "2026-10-01T23:30:00Z" };
    const when = "06:30, 02/10/2026";
    expect(text(5, params, locale)).toBe(
      locale === "vi"
        ? `${title} đã mở, đóng lúc ${when}`
        : `${title} is open and closes at ${when}`,
    );
    expect(text(6, params, locale)).toBe(
      locale === "vi" ? `${title} đóng lúc ${when}` : `${title} closes at ${when}`,
    );
    expect(text(7, params, locale)).toBe(
      locale === "vi"
        ? `${title} được gia hạn đến ${when}`
        : `${title} now closes at ${when}`,
    );
  });
});

function expectCollisionFreeTemplates(locale: "vi" | "en") {
  const templates: unknown = i18n.getResource(
    locale,
    "translation",
    "notifications.kind",
  );
  expect(templates).toBeTypeOf("object");
  expect(templates).not.toBeNull();
  const values = Object.values(templates as Record<string, unknown>);
  expect(values).toHaveLength(15);
  for (const value of values) {
    expect(value).toBeTypeOf("string");
    expect(value).not.toMatch(/\uE000quizzivy-notification:\d+\uE001/);
  }
}

describe("supported notification template invariant", () => {
  it.each(["vi", "en"] as const)(
    "keeps all 15 %s templates free of reserved tokens",
    (locale) => {
      expectCollisionFreeTemplates(locale);
    },
  );
});
