import { describe, expect, it } from "vitest";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec } from "@tests/support/openapi";
import { studentUser } from "@tests/support/fixtures";

const doc = loadSpec();
const ajv = new Ajv({ strict: false, validateFormats: true });
addFormats(ajv);
const current = ajv.compile({
  $ref: "#/components/schemas/CurrentUser",
  components: doc.components,
});
const publicUser = ajv.compile({
  $ref: "#/components/schemas/User",
  components: doc.components,
});
const patch = ajv.compile({
  ...doc.paths["/auth/me"].patch.requestBody.content["application/json"].schema,
  components: doc.components,
});
const preferences = ajv.compile({
  $ref: "#/components/schemas/UserPreferences",
  components: doc.components,
});
const publicBody = Object.fromEntries(
  Object.entries(studentUser).filter(
    ([key]) => key !== "permissions" && key !== "workspaces",
  ),
);

describe("profile and preference contract boundaries", () => {
  it("keeps six caller additions separate from two public additions", () => {
    const privateFields = {
      displayName: "Public",
      avatarUrl: "https://example.com/photo",
      phone: "+84 123456",
      locale: "en",
      timeZone: "UTC",
      preferences: { theme: "dark", compactTables: false },
    };
    expect(current({ ...studentUser, ...privateFields })).toBe(true);
    expect(
      publicUser({
        ...publicBody,
        displayName: "Public",
        avatarUrl: "https://example.com/photo",
      }),
    ).toBe(true);
    for (const field of ["phone", "locale", "timeZone", "preferences", "avatar_key"])
      expect(
        publicUser({
          ...publicBody,
          [field]: privateFields[field as keyof typeof privateFields] ?? "private",
        }),
      ).toBe(false);
    expect(current({ ...studentUser, avatar_key: "storage/key" })).toBe(false);
  });
  it("permits five partial inputs and nullable clears only where promised", () => {
    expect(
      Object.keys(
        doc.paths["/auth/me"].patch.requestBody.content["application/json"].schema
          .properties,
      ).sort((a, b) => a.localeCompare(b)),
    ).toEqual(["displayName", "fullName", "locale", "phone", "timeZone"]);
    expect(patch({ displayName: null, phone: null })).toBe(true);
    expect(
      patch({
        fullName: "Name",
        displayName: "Public",
        phone: "123456",
        locale: "vi",
        timeZone: "UTC",
      }),
    ).toBe(true);
    for (const body of [
      {},
      { fullName: null },
      { locale: null },
      { timeZone: null },
      { avatarKey: "x" },
      { preferences: {} },
    ])
      expect(patch(body)).toBe(false);
  });
  it("keeps preference objects closed while allowing false and empty replacements", () => {
    for (const body of [
      {},
      { compactTables: false, largerTestText: false },
      { assignmentDefaults: {} },
      { assignmentDefaults: { durationMinutes: 1 } },
      { assignmentDefaults: { durationMinutes: 600, showScore: false } },
    ])
      expect(preferences(body)).toBe(true);
    for (const body of [
      null,
      { theme: null },
      { theme: "neon" },
      { extra: true },
      { assignmentDefaults: null },
      { assignmentDefaults: { extra: true } },
      { assignmentDefaults: { durationMinutes: 0 } },
      { assignmentDefaults: { durationMinutes: 601 } },
    ])
      expect(preferences(body)).toBe(false);
  });
});
