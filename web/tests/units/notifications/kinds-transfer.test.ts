import { describe, expect, it } from "vitest";
import { FolderInput } from "lucide-react";
import i18n from "@/lib/i18n";
import { audienceOf, describeNotification } from "@/features/notifications/kinds";
import { listResponse, transferNotice } from "./support";

describe("the content transfer's notification kind", () => {
  it("validates against the contract", () => {
    expect(() =>
      listResponse({ items: [transferNotice], nextBefore: null }),
    ).not.toThrow();
  });
  it("words the hand-over in both languages and opens Classes in the teacher's bell", () => {
    const vi = describeNotification(transferNotice, i18n.getFixedT("vi"), "vi");
    const en = describeNotification(transferNotice, i18n.getFixedT("en"), "en");
    expect(vi?.text).toBe("Nội dung của Nguyễn Thị Lan đã được chuyển cho bạn: 42 mục");
    expect(en?.text).toBe("Nguyễn Thị Lan's content is now yours: 42 items");
    expect(en?.icon).toBe(FolderInput);
    expect(en?.tone).toBe("info");
    expect(en?.to).toBe("/teacher/classes");
    expect(audienceOf(transferNotice.kind)).toBe("teacher");
  });
  it("says one item in the singular", () => {
    const notice = { ...transferNotice, params: { teacherName: "Lan", count: 1 } };
    expect(describeNotification(notice, i18n.getFixedT("en"), "en")?.text).toBe(
      "Lan's content is now yours: 1 item",
    );
  });
  it("keeps a teacher's name that looks like markup or a template as plain text", () => {
    const notice = {
      ...transferNotice,
      params: { teacherName: "<b>{{count}}</b> $&", count: 2 },
    };
    expect(describeNotification(notice, i18n.getFixedT("en"), "en")?.text).toBe(
      "<b>{{count}}</b> $&'s content is now yours: 2 items",
    );
  });
  it.each([
    { count: 3 },
    { teacherName: "Lan" },
    { teacherName: "", count: 3 },
    { teacherName: "Lan", count: -1 },
  ])("skips params %j", (params) => {
    const notice = { ...transferNotice, params };
    expect(describeNotification(notice, i18n.getFixedT("vi"), "vi")).toBeNull();
  });
});
