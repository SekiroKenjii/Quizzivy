import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { http } from "msw";
import { uploadFile, __resetRefreshStateForTests } from "@/lib/api/client";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

beforeEach(() => {
  __resetRefreshStateForTests();
  useAuthStore.getState().setSession("token-a", adminUser);
});

afterEach(() => {
  vi.restoreAllMocks();
});

type Part = { form: FormData; name: string; value: unknown };

function recordForm() {
  const parts: Part[] = [];
  const append = FormData.prototype.append as (
    this: FormData,
    ...args: unknown[]
  ) => void;
  vi.spyOn(FormData.prototype, "append").mockImplementation(function (
    this: FormData,
    ...args: unknown[]
  ) {
    parts.push({ form: this, name: String(args[0]), value: args[1] });
    append.apply(this, args);
  } as never);
  server.use(
    http.post(`${BASE}/admin/settings/logo`, () =>
      contractJson("/admin/settings/logo", "post", 200, {
        sha256: "a".repeat(64),
        type: "image/png",
        width: 512,
        height: 512,
      }),
    ),
  );
  return parts;
}

describe("an upload's fields", () => {
  it("travel with the file as parts of the same form", async () => {
    const parts = recordForm();
    const file = new File(["x"], "mark.png", { type: "image/png" });

    await uploadFile("/admin/settings/logo", file, {
      fields: { variant: "dark", kind: "mark" },
    });

    expect(parts.map(({ name, value }) => [name, value])).toEqual([
      ["variant", "dark"],
      ["kind", "mark"],
      ["file", file],
    ]);
    expect(new Set(parts.map(({ form }) => form)).size).toBe(1);
  });

  it("come before the file, so a server that reads the stream meets them first", async () => {
    const parts = recordForm();

    await uploadFile("/admin/settings/logo", new File(["x"], "mark.png"), {
      fields: { variant: "light" },
    });

    expect(parts.map(({ name }) => name)).toEqual(["variant", "file"]);
  });

  it("are no part of the form when none are given", async () => {
    const parts = recordForm();

    await uploadFile("/admin/settings/logo", new File(["x"], "mark.png"));

    expect(parts.map(({ name }) => name)).toEqual(["file"]);
  });
});
