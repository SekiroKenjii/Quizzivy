import { describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";

describe("the bank's counted sentences", () => {
  it("speak of one question and one draft test in the singular", () => {
    const en = i18n.getFixedT("en");
    expect(en("bank.addToTestHint", { count: 1 })).toBe(
      "Choose the section to add the question to.",
    );
    expect(en("bank.addToTestHint", { count: 3 })).toBe(
      "Choose the section to add the 3 questions to.",
    );
    expect(en("bank.deleteBlockedList", { count: 1 })).toBe("Used in 1 draft test:");
    expect(en("bank.deleteBlockedList", { count: 2 })).toBe("Used in 2 draft tests:");
  });

  it("read the same for any count in Vietnamese", () => {
    const vi = i18n.getFixedT("vi");
    expect(vi("bank.addToTestHint", { count: 1 })).toBe(
      "Chọn phần muốn thêm 1 câu vào.",
    );
    expect(vi("bank.deleteBlockedList", { count: 1 })).toBe(
      "Đang dùng trong 1 đề nháp:",
    );
  });
});
