import { describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import en from "@/lib/i18n/locales/en.json";
import vi from "@/lib/i18n/locales/vi.json";
import { copyOf, PERMISSION_COPY } from "@/features/roles/permissionLabels";
import { loadSpec } from "@tests/support/openapi";
import { CATALOGUE } from "@tests/support/roles";

const HIDDEN = [
  "scope.all",
  "system.api_reference",
  "system.data_export",
  "system.leads",
];

function lookup(tree: unknown, path: string): unknown {
  return path
    .split(".")
    .reduce<unknown>(
      (node, part) =>
        typeof node === "object" && node !== null
          ? (node as Record<string, unknown>)[part]
          : undefined,
      tree,
    );
}

describe("the permission labels", () => {
  const keys: string[] = loadSpec().components.schemas.PermissionKey.enum;

  it("name every key of the contract but the four hidden ones", () => {
    expect(Object.keys(PERMISSION_COPY).sort((a, b) => a.localeCompare(b))).toEqual(
      keys.filter((key) => !HIDDEN.includes(key)).sort((a, b) => a.localeCompare(b)),
    );
  });

  it("have a string in both languages for every label and hint", () => {
    const missing: string[] = [];
    for (const copy of Object.values(PERMISSION_COPY)) {
      for (const path of [copy.label, copy.hint ?? null]) {
        if (path === null) continue;
        for (const [name, tree] of [
          ["vi", vi],
          ["en", en],
        ] as const) {
          if (typeof lookup(tree, path) !== "string") missing.push(`${name}:${path}`);
        }
      }
    }
    expect(missing).toEqual([]);
  });

  it("include D12's Create student accounts", () => {
    const copy = copyOf("people.students.create");
    expect(copy).not.toBeNull();
    expect(i18n.t(copy!.label, { lng: "vi" })).toBe("Tạo tài khoản học viên");
    expect(i18n.t(copy!.label, { lng: "en" })).toBe("Create student accounts");
  });

  it("cover every row the server's catalogue draws", () => {
    for (const entry of CATALOGUE) expect(copyOf(entry.key)).not.toBeNull();
  });

  it("give no row to a hidden key", () => {
    for (const key of HIDDEN) expect(copyOf(key as never)).toBeNull();
  });
});
