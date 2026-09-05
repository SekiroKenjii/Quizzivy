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

describe("MSW fixtures are validated against api/openapi.yaml", () => {
  it("accepts a body that matches the contract", () => {
    expect(() => contractJson("/auth/me", "get", 200, studentUser)).not.toThrow();
  });

  it("rejects a missing required field", () => {
    const withoutEmail = Object.fromEntries(
      Object.entries(studentUser).filter(([key]) => key !== "email"),
    );
    expect(() => contractJson("/auth/me", "get", 200, withoutEmail)).toThrow(/email/);
  });

  it("rejects a field the contract does not define", () => {
    expect(() =>
      contractJson("/auth/me", "get", 200, { ...studentUser, isCorrect: true }),
    ).toThrow(/isCorrect|additional/i);
  });

  it("rejects a wrong type", () => {
    expect(() =>
      contractJson("/auth/me", "get", 200, { ...studentUser, hasPassword: "yes" }),
    ).toThrow(/hasPassword/);
  });

  it("rejects a value outside an enum", () => {
    expect(() =>
      contractJson("/auth/me", "get", 200, { ...studentUser, role: "superuser" }),
    ).toThrow(/role/);
  });

  it("rejects a response the contract never declares", () => {
    expect(() => contractJson("/auth/me", "get", 418, studentUser)).toThrow(
      /defines no such response/,
    );
  });

  it("names the offending field, so the failure is actionable", () => {
    let message = "";
    try {
      contractJson("/app/classes", "get", 200, { items: [{ id: "not-a-uuid" }] });
    } catch (e) {
      message = (e as Error).message;
    }
    expect(message).toContain("/app/classes");
    expect(message).toMatch(/items/);
  });
});
