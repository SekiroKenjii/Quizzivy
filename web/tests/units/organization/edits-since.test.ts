import { describe, expect, it } from "vitest";
import { editsSince } from "@/features/organization/editsSince";

const FORM = {
  organization: {
    name: "Quizzivy",
    address: null,
    logo: { light: { sha256: "a", type: "image/png" }, dark: null },
  },
  signIn: { google: true, allowedGoogleDomains: ["quizzivy.com"] },
  retention: { keepAnswersMonths: 24 },
};

describe("editsSince", () => {
  it("lists nothing when the form is as it was sent", () => {
    expect(editsSince(FORM, structuredClone(FORM))).toEqual([]);
  });

  it("names each changed field by its path, with the value it holds now", () => {
    const now = structuredClone(FORM);
    now.organization.address = "12 Nguyễn Huệ" as never;
    now.signIn.google = false;
    now.retention.keepAnswersMonths = 12;

    expect(editsSince(FORM, now)).toEqual([
      { path: "organization.address", value: "12 Nguyễn Huệ" },
      { path: "signIn.google", value: false },
      { path: "retention.keepAnswersMonths", value: 12 },
    ]);
  });

  it("treats a list of domains as one field", () => {
    const now = structuredClone(FORM);
    now.signIn.allowedGoogleDomains = ["quizzivy.com", "gmail.com"];

    expect(editsSince(FORM, now)).toEqual([
      {
        path: "signIn.allowedGoogleDomains",
        value: ["quizzivy.com", "gmail.com"],
      },
    ]);
  });

  it("compares a logo that was set by its own fields, and a null logo as a whole", () => {
    const replaced = structuredClone(FORM);
    replaced.organization.logo.light.sha256 = "b";
    const added = structuredClone(FORM);
    added.organization.logo.dark = { sha256: "c", type: "image/png" } as never;

    expect(editsSince(FORM, replaced)).toEqual([
      { path: "organization.logo.light.sha256", value: "b" },
    ]);
    expect(editsSince(FORM, added)).toEqual([
      {
        path: "organization.logo.dark",
        value: { sha256: "c", type: "image/png" },
      },
    ]);
  });
});
