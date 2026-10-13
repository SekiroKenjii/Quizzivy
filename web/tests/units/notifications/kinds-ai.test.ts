import { describe, expect, it } from "vitest";
import { Hand, Sparkles } from "lucide-react";
import i18n from "@/lib/i18n";
import { audienceOf, describeNotification } from "@/features/notifications/kinds";
import { listResponse, aiNotices } from "./support";

const expected = [
  [
    "Bạn đã dùng 16.500 trên 20.000 tín dụng AI trong tháng này",
    "You have used 16,500 of your 20,000 AI credits this month",
    Sparkles,
    "warning",
  ],
  [
    "Nguyễn Thị Lan xin thêm 20.000 tín dụng AI",
    "Nguyễn Thị Lan asked for 20,000 more AI credits",
    Hand,
    "warning",
  ],
  [
    "Yêu cầu thêm 20.000 tín dụng AI của bạn đã được duyệt",
    "Your request for 20,000 more AI credits was approved",
    Sparkles,
    "info",
  ],
  [
    "Yêu cầu thêm 5.000 tín dụng AI của bạn đã bị từ chối",
    "Your request for 5,000 more AI credits was declined",
    Sparkles,
    "info",
  ],
] as const;

describe("the assistant's notification kinds", () => {
  it("validate against the contract", () => {
    expect(() => listResponse({ items: aiNotices, nextBefore: null })).not.toThrow();
  });
  it.each(aiNotices.map((notice, index) => ({ notice, index })))(
    "words $notice.kind in both languages, with no destination, for the teacher's bell",
    ({ notice, index }) => {
      for (const locale of ["vi", "en"] as const) {
        const view = describeNotification(notice, i18n.getFixedT(locale), locale);
        expect(view?.text).toBe(expected[index]![locale === "vi" ? 0 : 1]);
        expect(view?.icon).toBe(expected[index]![2]);
        expect(view?.tone).toBe(expected[index]![3]);
        expect(view?.to).toBeNull();
      }
      expect(audienceOf(notice.kind)).toBe("teacher");
    },
  );
  it("keeps a teacher's name that looks like markup or a template as plain text", () => {
    const notice = {
      ...aiNotices[1]!,
      params: { teacherName: "<b>{{credits}}</b> $&", credits: 1_000 },
    };
    expect(describeNotification(notice, i18n.getFixedT("en"), "en")?.text).toBe(
      "<b>{{credits}}</b> $& asked for 1,000 more AI credits",
    );
  });
  it.each([
    [0, { used: 16_500 }],
    [0, { allowance: 20_000 }],
    [0, { used: -1, allowance: 20_000 }],
    [1, { credits: 20_000 }],
    [1, { teacherName: "Lan" }],
    [1, { teacherName: "", credits: 20_000 }],
    [2, { credits: 20_000 }],
    [2, { credits: 20_000, decision: "pending" }],
    [2, { decision: "approved" }],
  ] as const)("skips fixture%s whose params are %j", (index, params) => {
    const notice = { ...aiNotices[index]!, params } as (typeof aiNotices)[number];
    expect(describeNotification(notice, i18n.getFixedT("vi"), "vi")).toBeNull();
  });
});
