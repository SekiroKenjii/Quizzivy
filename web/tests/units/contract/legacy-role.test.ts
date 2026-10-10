import { describe, expect, it } from "vitest";
import Ajv from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { loadSpec } from "@tests/support/openapi";
import { adminUser, studentUser } from "@tests/support/fixtures";

const doc = loadSpec();
const ajv = new Ajv({ strict: false, validateFormats: true });
addFormats(ajv);
const current = ajv.compile({
  $ref: "#/components/schemas/CurrentUser",
  components: doc.components,
});

describe("the legacy role is out of the contract", () => {
  it("has no Role schema", () => {
    expect(doc.components.schemas.Role).toBeUndefined();
  });

  it.each(["User", "CurrentUser"])("%s neither defines nor requires a role", (name) => {
    const schema = doc.components.schemas[name];
    expect(schema.properties.role).toBeUndefined();
    expect(schema.required).not.toContain("role");
  });

  it.each([
    ["a student", studentUser],
    ["an admin", adminUser],
  ])("refuses a signed-in user body that carries role for %s", (_name, user) => {
    expect(current(user)).toBe(true);
    expect(current({ ...user, role: "student" })).toBe(false);
  });
});
