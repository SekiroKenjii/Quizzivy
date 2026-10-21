import { afterEach, describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { roleDescription, roleName } from "@/lib/roleName";

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

const seededDescription = {
  admin: "Điều hành trung tâm. Quản lý người dùng và cài đặt.",
  teacher: "Soạn đề, quản lý lớp, chấm bài.",
  assistant: "Hỗ trợ giáo viên chấm bài và điểm danh.",
  student: "Làm bài và xem kết quả của mình.",
} as const;

const description = (builtinKey: keyof typeof seeded | null, stored: string) =>
  roleDescription({ builtinKey, description: stored }, i18n.t);

describe("a built-in role whose stored description is still the seeded one", () => {
  it.each(
    Object.entries(seededDescription) as [keyof typeof seededDescription, string][],
  )("reads %s as the seed, to the character, in Vietnamese", (key, stored) => {
    expect(description(key, stored)).toBe(stored);
  });

  it.each([
    ["admin", "Runs the centre. Manages people and settings."],
    ["teacher", "Builds tests, runs classes, grades."],
    ["assistant", "Helps teachers grade and take attendance."],
    ["student", "Takes tests and sees their own results."],
  ] as const)("reads %s in English", async (key, english) => {
    await i18n.changeLanguage("en");
    expect(description(key, seededDescription[key])).toBe(english);
  });
});

describe("a role whose stored description is its own", () => {
  it("shows an edited built-in description as stored, in either language", async () => {
    expect(description("teacher", "Dẫn dắt nhóm IELTS")).toBe("Dẫn dắt nhóm IELTS");
    await i18n.changeLanguage("en");
    expect(description("teacher", "Dẫn dắt nhóm IELTS")).toBe("Dẫn dắt nhóm IELTS");
  });

  it("shows a custom role's description as stored, even when it is a built-in role's seed", async () => {
    await i18n.changeLanguage("en");
    expect(description(null, seededDescription.teacher)).toBe(
      seededDescription.teacher,
    );
  });

  it("shows an empty description as empty", async () => {
    await i18n.changeLanguage("en");
    expect(description("student", "")).toBe("");
  });

  it("does not take another built-in role's seed for its own", async () => {
    await i18n.changeLanguage("en");
    expect(description("student", seededDescription.teacher)).toBe(
      seededDescription.teacher,
    );
  });
});
