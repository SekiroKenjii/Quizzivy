import { describe, expect, it } from "vitest";
import { contractJson } from "@tests/support/contractResponse";
import { studentUser } from "@tests/support/fixtures";

/**
 * Proves the mock-validation layer can actually fail.
 *
 * Without this, `contractJson` could silently degrade into `HttpResponse.json`
 * — every handler would still return, every test would still pass, and the
 * protection would be gone with nothing to show for it.
 */

function refusal(body: unknown, path = "/auth/me", status = 200): string {
  try {
    contractJson(path, "get", status, body);
  } catch (e) {
    return (e as Error).message.split("\n\nBody:")[0]!;
  }
  throw new Error(`the contract accepted ${JSON.stringify(body)}`);
}

describe("MSW fixtures are validated against api/openapi.yaml", () => {
  it("accepts a body that matches the contract", () => {
    expect(() => contractJson("/auth/me", "get", 200, studentUser)).not.toThrow();
  });

  it("rejects a missing required field", () => {
    const withoutEmail = Object.fromEntries(
      Object.entries(studentUser).filter(([key]) => key !== "email"),
    );
    expect(refusal(withoutEmail)).toContain("must have required property 'email'");
  });

  it("rejects a field the contract does not define", () => {
    expect(refusal({ ...studentUser, isCorrect: true })).toContain(
      "must NOT have additional properties",
    );
  });

  it("rejects a wrong type", () => {
    expect(refusal({ ...studentUser, hasPassword: "yes" })).toContain(
      "/hasPassword must be boolean",
    );
  });

  it("rejects a value outside an enum", () => {
    expect(refusal({ ...studentUser, locale: "klingon" })).toContain(
      "/locale must be equal to one of the allowed values",
    );
  });

  it("rejects a response the contract never declares", () => {
    expect(() => contractJson("/auth/me", "get", 418, studentUser)).toThrow(
      /defines no such response/,
    );
  });

  it("names the offending field, so the failure is actionable", () => {
    const message = refusal({ items: [{ id: "not-a-uuid" }] }, "/app/classes");
    expect(message).toContain("/app/classes");
    expect(message).toMatch(/\/items\/0/);
  });
});
