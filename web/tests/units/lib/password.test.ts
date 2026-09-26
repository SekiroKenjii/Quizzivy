import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { parse } from "yaml";

import { newPasswordSchema } from "@/features/auth/changePasswordSchema";
import { passwordRules, passwordStrength } from "@/lib/password";

const contract = parse(
  readFileSync(resolve(import.meta.dirname, "../../../../api/openapi.yaml"), "utf8"),
) as {
  paths: Record<string, Record<string, unknown>>;
};

function newPasswordRule(): { minLength: number; pattern: string } {
  const post = contract.paths["/auth/change-password"]?.["post"] as {
    requestBody: {
      content: {
        "application/json": {
          schema: {
            properties: { newPassword: { minLength: number; pattern: string } };
          };
        };
      };
    };
  };
  return post.requestBody.content["application/json"].schema.properties.newPassword;
}

describe("the new-password rules", () => {
  it("are the contract's", () => {
    const rule = newPasswordRule();
    const pattern = new RegExp(rule.pattern, "u");
    for (const password of [
      "matkhau1",
      "mật khẩu!",
      "mậtkhẩuđẹp",
      "abc",
      "12345678",
      "a+b=c",
    ]) {
      const contractSays =
        [...password].length >= rule.minLength && pattern.test(password);
      const rules = passwordRules(password);
      expect(rules.length && rules.numberOrSymbol, password).toBe(contractSays);
      expect(newPasswordSchema.safeParse(password).success, password).toBe(
        contractSays,
      );
    }
  });

  it("count characters, not UTF-16 units", () => {
    expect(passwordRules("😀😀😀😀1")).toMatchObject({ length: false });
    expect(passwordRules("😀😀😀😀1234")).toMatchObject({ length: true });
  });

  it("name the first rule broken", () => {
    const short = newPasswordSchema.safeParse("abc");
    expect(short.error?.issues[0]?.message).toBe("changePassword.errors.tooShort");
    const letters = newPasswordSchema.safeParse("mậtkhẩuđẹp");
    expect(letters.error?.issues[0]?.message).toBe(
      "changePassword.errors.numberOrSymbol",
    );
  });
});

describe("passwordStrength", () => {
  it("scores the deck's meter", () => {
    expect(passwordStrength("")).toBe(0);
    expect(passwordStrength("abc")).toBe(1);
    expect(passwordStrength("abcdefgh")).toBe(2);
    expect(passwordStrength("abcdefg1")).toBe(3);
    expect(passwordStrength("abcdefghijk1")).toBe(4);
  });
});
