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

function sent(handler: (form: FormData) => void) {
  server.use(
    http.post(`${BASE}/admin/settings/logo`, async ({ request }) => {
      handler(await request.formData());
      return contractJson("/admin/settings/logo", "post", 200, {
        sha256: "a".repeat(64),
        type: "image/png",
        width: 512,
        height: 512,
      });
    }),
  );
}

describe("an upload's fields", () => {
  it("travel with the file as parts of the same form", async () => {
    let seen: Record<string, string> = {};
    sent((form) => {
      seen = { variant: String(form.get("variant")), kind: String(form.get("kind")) };
    });

    await uploadFile(
      "/admin/settings/logo",
      new File(["x"], "mark.png", { type: "image/png" }),
      {
        fields: { variant: "dark", kind: "mark" },
      },
    );

    expect(seen).toEqual({ variant: "dark", kind: "mark" });
  });

  it("come before the file, so a server that reads the stream meets them first", async () => {
    const order: string[] = [];
    const append = FormData.prototype.append as (
      this: FormData,
      ...args: unknown[]
    ) => void;
    vi.spyOn(FormData.prototype, "append").mockImplementation(function (
      this: FormData,
      ...args: unknown[]
    ) {
      order.push(String(args[0]));
      append.apply(this, args);
    } as never);
    sent(() => undefined);

    await uploadFile("/admin/settings/logo", new File(["x"], "mark.png"), {
      fields: { variant: "light" },
    });

    expect(order).toEqual(["variant", "file"]);
  });

  it("are no part of the form when none are given", async () => {
    let names: string[] = [];
    sent((form) => {
      names = [...form.keys()];
    });

    await uploadFile("/admin/settings/logo", new File(["x"], "mark.png"));

    expect(names).toEqual(["file"]);
  });
});
