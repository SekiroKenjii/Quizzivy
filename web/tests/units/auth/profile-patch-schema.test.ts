import { describe, expect, it } from "vitest";
import { profilePatchSchema, profileSchema } from "@/features/auth/profileSchema";

describe("complete partial profile schema", () => {
  it("accepts exactly the five optional fields without supplying omitted values", () => {
    const fields = {
      fullName: "Tên",
      displayName: "Cô An",
      phone: "+84 123456",
      locale: "en",
      timeZone: "UTC",
    };
    expect(profilePatchSchema.parse(fields)).toEqual(fields);
    expect(profilePatchSchema.parse({ displayName: null })).toEqual({
      displayName: null,
    });
    expect(profilePatchSchema.parse({ phone: null })).toEqual({ phone: null });
    expect(profilePatchSchema.parse({ locale: "vi", phone: undefined })).toEqual({
      locale: "vi",
    });
  });
  it.each([
    {},
    { fullName: undefined },
    { fullName: null },
    { locale: null },
    { timeZone: null },
    { preferences: {} },
    { avatarUrl: "https://example.com/a" },
    { unknown: "x" },
    { locale: "fr" },
    { timeZone: "" },
    { timeZone: "Local" },
    { timeZone: " " },
    { displayName: " " },
    { phone: "123-456" },
  ])("refuses malformed or empty request %j", (patch) => {
    expect(profilePatchSchema.safeParse(patch).success).toBe(false);
  });
  it("uses raw Unicode bounds and normalized nonempty names", () => {
    expect(
      profilePatchSchema.safeParse({
        fullName: "ữ".repeat(200),
        displayName: "😀".repeat(80),
      }).success,
    ).toBe(true);
    expect(profilePatchSchema.safeParse({ fullName: "ữ".repeat(201) }).success).toBe(
      false,
    );
    expect(profilePatchSchema.safeParse({ displayName: "😀".repeat(81) }).success).toBe(
      false,
    );
    expect(
      profilePatchSchema.safeParse({ fullName: " " + "ữ".repeat(200) }).success,
    ).toBe(false);
    expect(profilePatchSchema.safeParse({ fullName: "\t " }).success).toBe(false);
    expect(profilePatchSchema.parse({ phone: "      " })).toEqual({ phone: "      " });
  });
  it("preserves the current required-name form and localized errors", () => {
    expect(profileSchema.parse({ fullName: "  Nguyễn An  " })).toEqual({
      fullName: "Nguyễn An",
    });
    const missing = profileSchema.safeParse({ fullName: " " });
    expect(missing.success).toBe(false);
    if (!missing.success)
      expect(missing.error.issues[0]?.message).toBe("settings.errors.nameRequired");
    expect(profileSchema.safeParse({}).success).toBe(false);
  });
});
