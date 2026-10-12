import { expect, test } from "vitest";
import { loadSpec } from "@tests/support/openapi";

test("self preferences has the raw8192 inclusive cap with declared400 mapping", () => {
  const doc = loadSpec();
  const operation = doc.paths["/me/preferences"].patch;
  expect(operation["x-max-body-bytes"]).toBe(8192);
  expect(operation["x-body-limit-status"]).toBe(400);
  expect(operation["x-permission"]).toBe("self");
  expect(operation.responses[400]).toBeDefined();
  expect(operation.requestBody.content["application/json"].schema.$ref).toBe(
    "#/components/schemas/UserPreferences",
  );
  expect(doc.paths["/teacher/media"].post["x-body-limit-status"]).toBeUndefined();
});
