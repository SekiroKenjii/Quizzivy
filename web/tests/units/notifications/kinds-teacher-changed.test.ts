import { describe, expect, it } from "vitest";
import { UserRoundCog } from "lucide-react";
import i18n from "@/lib/i18n";
import { audienceOf, describeNotification } from "@/features/notifications/kinds";
import type { Notification } from "@/features/notifications/api";
import { listResponse, teacherChangedNotices } from "./support";

const [received, changed] = teacherChangedNotices as [Notification, Notification];

describe("the change of a class's teacher's notification kind", () => {
  it("validates against the contract with and without the new teacher's name", () => {
    expect(() =>
      listResponse({ items: teacherChangedNotices, nextBefore: null }),
    ).not.toThrow();
  });
  it("tells the new teacher the class is theirs in both languages", () => {
    const vi = describeNotification(received, i18n.getFixedT("vi"), "vi");
    const en = describeNotification(received, i18n.getFixedT("en"), "en");
    expect(vi?.text).toBe("Bạn được giao dạy lớp IELTS Foundation A");
    expect(en?.text).toBe("You now teach IELTS Foundation A");
    expect(en?.icon).toBe(UserRoundCog);
    expect(en?.tone).toBe("info");
    expect(en?.to).toBe("/teacher/classes");
    expect(audienceOf(received.kind)).toBe("teacher");
  });
  it("tells the previous teacher who teaches the class now", () => {
    expect(describeNotification(changed, i18n.getFixedT("vi"), "vi")?.text).toBe(
      "Lớp IELTS Foundation A đã được giao cho Trần Minh Khoa",
    );
    expect(describeNotification(changed, i18n.getFixedT("en"), "en")?.text).toBe(
      "IELTS Foundation A is now taught by Trần Minh Khoa",
    );
  });
  it("keeps names that look like markup or a template as plain text", () => {
    const notice = {
      ...changed,
      params: { className: "<b>{{teacherName}}</b>", teacherName: "$& {{className}}" },
    };
    expect(describeNotification(notice, i18n.getFixedT("en"), "en")?.text).toBe(
      "<b>{{teacherName}}</b> is now taught by $& {{className}}",
    );
  });
  it.each([
    {},
    { className: "" },
    { className: "Lớp A", teacherName: "" },
    { teacherName: "Khoa" },
  ])("skips params %j", (params) => {
    const notice = { ...changed, params };
    expect(describeNotification(notice, i18n.getFixedT("vi"), "vi")).toBeNull();
  });
});
