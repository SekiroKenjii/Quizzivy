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

  it("User neither defines nor requires a role", () => {
    const schema = doc.components.schemas.User;
    expect(schema.properties.role).toBeUndefined();
    expect(schema.required).not.toContain("role");
  });

  it("CurrentUser's role is the caller's own role record, optional", () => {
    const schema = doc.components.schemas.CurrentUser;
    expect(schema.properties.role).toEqual({
      $ref: "#/components/schemas/CurrentUserRole",
    });
    expect(schema.required).not.toContain("role");
  });

  it.each([
    ["a student", studentUser],
    ["an admin", adminUser],
  ])(
    "refuses a signed-in user body that carries the legacy role string for %s",
    (_name, user) => {
      expect(current(user)).toBe(true);
      expect(current({ ...user, role: "student" })).toBe(false);
    },
  );

  it.each([
    ["a built-in role", { builtinKey: "admin", name: "Quản trị viên" }],
    ["a custom role", { builtinKey: null, name: "Trợ lý phòng đề" }],
  ])("takes %s as the role record", (_name, role) => {
    const user = {
      ...adminUser,
      role: { id: "019535d9-3df7-79fb-b466-fa907fa17f9e", ...role },
    };
    expect(current(user)).toBe(true);
    expect(current({ ...user, role: { ...user.role, description: "x" } })).toBe(false);
    expect(current({ ...user, role: { id: user.role.id, name: user.role.name } })).toBe(
      false,
    );
  });
});
