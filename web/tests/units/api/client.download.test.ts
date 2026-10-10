import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  api,
  downloadFile,
  setSessionLostHandler,
  __resetRefreshStateForTests,
} from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";

/**
 * downloadFile shares `api`'s token and its one refresh (R-06): a file
 * request that meets a 401 waits on the same refresh as every JSON request,
 * and a refusal reads as the same ApiError.
 */

type Handler = (url: string, init: RequestInit) => Response | Promise<Response>;

const CSV = "/teacher/assignments/results.csv";
const IDS = { ids: ["018f0000-0000-7000-8000-0000000000d1"] };

let calls: { url: string; auth: string | null }[] = [];
let handler: Handler;

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function envelope(code: string, status: number, message = "boom") {
  return json({ error: { code, message, requestId: "req-1" } }, status);
}

function csv(disposition?: string) {
  return new Response("Assignment,Student\r\n", {
    status: 200,
    headers: {
      "Content-Type": "text/csv",
      ...(disposition === undefined ? {} : { "Content-Disposition": disposition }),
    },
  });
}

const refreshCalls = () => calls.filter((call) => call.url.endsWith("/auth/refresh"));

beforeEach(() => {
  calls = [];
  __resetRefreshStateForTests();
  useAuthStore.getState().clearSession();
  useAuthStore.getState().setAccessToken("initial-token");
  setSessionLostHandler(() => {});
  vi.stubGlobal("fetch", (input: string | URL, init: RequestInit = {}) => {
    const url = String(input);
    calls.push({ url, auth: new Headers(init.headers).get("Authorization") });
    return handler(url, init);
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("downloadFile", () => {
  it("refreshes once after a 401 and retries with the new token", async () => {
    let refreshed = false;
    handler = (url) => {
      if (url.endsWith("/auth/refresh")) {
        refreshed = true;
        return json({ accessToken: "fresh-token", expiresIn: 900 });
      }
      return refreshed
        ? csv('attachment; filename="results-20260829.csv"')
        : envelope("X", 401);
    };

    const file = await downloadFile(CSV, IDS);

    expect(await file.blob.text()).toBe("Assignment,Student\r\n");
    expect(refreshCalls()).toHaveLength(1);
    expect(
      calls.filter((call) => call.url.includes(CSV)).map((call) => call.auth),
    ).toEqual(["Bearer initial-token", "Bearer fresh-token"]);
    expect(calls[0]?.url).toContain(`ids=${IDS.ids[0]}`);
  });

  it("loses the session once when the refresh fails", async () => {
    const lost = vi.fn();
    setSessionLostHandler(lost);
    handler = () => envelope("REFRESH_TOKEN_INVALID", 401);

    await expect(downloadFile(CSV, IDS)).rejects.toBeInstanceOf(ApiError);
    expect(lost).toHaveBeenCalledOnce();
    expect(useAuthStore.getState().accessToken).toBeNull();
  });

  it("shares one refresh with an api() call that meets a 401 at the same time", async () => {
    let refreshed = false;
    handler = (url) => {
      if (url.endsWith("/auth/refresh")) {
        refreshed = true;
        return json({ accessToken: "fresh-token", expiresIn: 900 });
      }
      if (!refreshed) return envelope("X", 401);
      return url.includes(CSV)
        ? csv()
        : json({ items: [], page: 1, pageSize: 50, total: 0 });
    };

    await Promise.all([downloadFile(CSV, IDS), api("get", "/teacher/classes")]);

    expect(refreshCalls()).toHaveLength(1);
  });

  it("throws a refusal's error envelope as the ApiError api() throws", async () => {
    handler = () => envelope("RATE_LIMITED", 429, "Thử lại sau ít phút.");

    const download = await downloadFile(CSV, IDS).catch((cause: unknown) => cause);
    const json_ = await api("get", "/teacher/classes").catch((cause: unknown) => cause);

    expect(download).toBeInstanceOf(ApiError);
    expect(download).toMatchObject({
      status: 429,
      code: "RATE_LIMITED",
      message: "Thử lại sau ít phút.",
    });
    expect({
      ...(download as ApiError),
      message: (download as ApiError).message,
    }).toEqual({
      ...(json_ as ApiError),
      message: (json_ as ApiError).message,
    });
  });

  it("names the file by filename* before filename, as RFC 6266 asks", async () => {
    handler = () =>
      csv(
        `attachment; filename="ket-qua.csv"; filename*=UTF-8''k%E1%BA%BFt-qu%E1%BA%A3.csv`,
      );
    expect((await downloadFile(CSV, IDS)).filename).toBe("kết-quả.csv");
  });

  it("names the file by filename when there is no filename*", async () => {
    handler = () => csv('attachment; filename="results-20260829.csv"');
    expect((await downloadFile(CSV, IDS)).filename).toBe("results-20260829.csv");
  });

  it("gives no name when the response names none", async () => {
    handler = () => csv();
    expect((await downloadFile(CSV, IDS)).filename).toBeNull();
  });
});
