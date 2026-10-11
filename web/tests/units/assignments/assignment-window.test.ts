import { afterEach, describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { moment } from "@/features/assignments/pages/teacher/assignmentWindow";

const NOW = new Date("2026-10-11T02:00:00Z");
const TONIGHT = "2026-10-11T14:00:00Z";
const TOMORROW = "2026-10-12T01:00:00Z";

afterEach(() => void i18n.changeLanguage("vi"));

describe("moment (F-60)", () => {
  it("keeps the day's capital where the moment starts a line", () => {
    expect(moment(TONIGHT, NOW, i18n.t)).toBe("Hôm nay, 21:00");
    expect(moment(TOMORROW, NOW, i18n.t)).toBe("Ngày mai, 08:00");
  });

  it("lowercases the day inside a sentence", () => {
    expect(moment(TONIGHT, NOW, i18n.t, true)).toBe("hôm nay, 21:00");
    expect(moment(TOMORROW, NOW, i18n.t, true)).toBe("ngày mai, 08:00");
    expect(
      i18n.t("assignments.list.extendWhen", {
        title: "Unit 5",
        when: moment(TONIGHT, NOW, i18n.t, true),
      }),
    ).toBe("Unit 5 · đóng hôm nay, 21:00");
  });

  it("does the same in English", async () => {
    await i18n.changeLanguage("en");
    expect(moment(TONIGHT, NOW, i18n.t)).toBe("Today, 21:00");
    expect(moment(TONIGHT, NOW, i18n.t, true)).toBe("today, 21:00");
  });
});
