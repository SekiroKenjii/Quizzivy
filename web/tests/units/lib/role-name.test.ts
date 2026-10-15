import { afterEach, describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { roleName } from "@/lib/roleName";

const seeded = {
  admin: "Quản trị viên",
  teacher: "Giáo viên",
  assistant: "Trợ giảng",
  student: "Học viên",
} as const;

const name = (builtinKey: keyof typeof seeded | null, stored: string) =>
  roleName({ builtinKey, name: stored }, i18n.t);

afterEach(() => i18n.changeLanguage("vi"));

describe("a built-in role whose stored name is still the seeded one", () => {
  it.each(Object.entries(seeded) as [keyof typeof seeded, string][])(
    "reads %s as the seeded Vietnamese name",
    (key, stored) => {
      expect(name(key, stored)).toBe(stored);
    },
  );

  it.each([
    ["admin", "Admin"],
    ["teacher", "Teacher"],
    ["assistant", "Assistant"],
    ["student", "Student"],
  ] as const)("reads %s in English", async (key, english) => {
    await i18n.changeLanguage("en");
    expect(name(key, seeded[key])).toBe(english);
  });
});

describe("a role whose stored name is its own", () => {
  it("shows a renamed built-in role as stored, in either language", async () => {
    expect(name("admin", "Giám đốc")).toBe("Giám đốc");
    await i18n.changeLanguage("en");
    expect(name("admin", "Giám đốc")).toBe("Giám đốc");
  });

  it("shows a custom role as stored, even when it is named like a built-in one", async () => {
    expect(name(null, "Trợ lý phòng đề")).toBe("Trợ lý phòng đề");
    await i18n.changeLanguage("en");
    expect(name(null, seeded.teacher)).toBe(seeded.teacher);
  });

  it("does not take another built-in role's seeded name for its own", () => {
    expect(name("student", seeded.teacher)).toBe(seeded.teacher);
  });
});
