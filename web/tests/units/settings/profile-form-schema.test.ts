import { describe, expect, it } from "vitest";
import { profileFormSchema } from "@/features/settings/profileFormSchema";

const BASE = {
  fullName: " Hoàng Thương ",
  displayName: "Ms Thương",
  phone: "0912 345 678",
  locale: "vi" as const,
  timeZone: "Asia/Ho_Chi_Minh",
};

function issue(values: Partial<typeof BASE>) {
  return profileFormSchema.safeParse({ ...BASE, ...values }).error?.issues[0]?.message;
}

describe("the Profile form's rules", () => {
  it("trims the names and sends the whole profile", () => {
    expect(profileFormSchema.parse(BASE)).toEqual({
      ...BASE,
      fullName: "Hoàng Thương",
    });
  });

  it("clears a blank name students see or phone", () => {
    expect(
      profileFormSchema.parse({ ...BASE, displayName: "  ", phone: "" }),
    ).toMatchObject({ displayName: null, phone: null });
  });

  it("names the rule a field breaks", () => {
    expect(issue({ fullName: " " })).toBe("settings.errors.nameRequired");
    expect(issue({ fullName: "x".repeat(201) })).toBe("settings.errors.nameTooLong");
    expect(issue({ displayName: "x".repeat(81) })).toBe(
      "settings.errors.displayNameTooLong",
    );
    expect(issue({ phone: "09-12" })).toBe("settings.errors.phoneInvalid");
  });
});
