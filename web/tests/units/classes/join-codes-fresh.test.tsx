import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { http } from "msw";
import { joinCodesQuery, useJoinCodesOff } from "@/features/organization/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";

const BASE = "http://localhost:8080";

function organization(joinCodes: boolean) {
  return {
    name: "Quizzivy",
    shortName: null,
    website: null,
    contactEmail: null,
    frontDeskPhone: null,
    frontDeskHours: null,
    address: null,
    logo: { lightUrl: null, darkUrl: null },
    signIn: { google: true, password: true },
    joinCodes,
  };
}

function setup(agedBy: number) {
  const reads: RequestCache[] = [];
  server.use(
    http.get(`${BASE}/public/organization`, ({ request }) => {
      reads.push(request.cache);
      return contractJson("/public/organization", "get", 200, organization(false));
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  });
  client.setQueryData(joinCodesQuery().queryKey, organization(true), {
    updatedAt: Date.now() - agedBy,
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { reads, hook: renderHook(() => useJoinCodesOff(), { wrapper }) };
}

describe("useJoinCodesOff follows the centre's setting within the minute (QC-U-18)", () => {
  it("asks again when its answer is a minute old, bypassing the browser's five-minute cache", async () => {
    const { reads, hook } = setup(60_000);
    await waitFor(() => expect(hook.result.current).toBe(true));
    expect(reads).toEqual(["no-cache"]);
  });

  it("keeps a fresh answer without asking", async () => {
    const { reads, hook } = setup(5_000);
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(hook.result.current).toBe(false);
    expect(reads).toEqual([]);
  });
});

describe("useJoinCodesOff follows the setting when the tab is focused again (QC-U-20)", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  function focusAgain() {
    act(() => {
      window.dispatchEvent(new Event("visibilitychange"));
    });
  }

  it("asks again on focus once thirty seconds have passed, and not before", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    const { reads, hook } = setup(20_000);
    expect(hook.result.current).toBe(false);

    focusAgain();
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(reads).toEqual([]);
    expect(hook.result.current).toBe(false);

    vi.setSystemTime(Date.now() + 31_000);
    focusAgain();
    await waitFor(() => expect(hook.result.current).toBe(true));
    expect(reads).toEqual(["no-cache"]);
  });
});
