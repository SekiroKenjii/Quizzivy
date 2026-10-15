import { describe, expect, it } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { http, HttpResponse } from "msw";
import { passwordSignInQuery, usePasswordSignIn } from "@/features/organization/api";
import { BASE_URL } from "@/lib/api/client";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";

function organization(password: boolean) {
  return {
    name: "Quizzivy",
    shortName: null,
    website: null,
    contactEmail: null,
    frontDeskPhone: null,
    frontDeskHours: null,
    address: null,
    logo: { lightUrl: null, darkUrl: null },
    signIn: { google: true, password },
    joinCodes: true,
  };
}

function setup(agedBy: number) {
  const reads: RequestCache[] = [];
  server.use(
    http.get(`${BASE_URL}/public/organization`, ({ request }) => {
      reads.push(request.cache);
      return contractJson("/public/organization", "get", 200, organization(false));
    }),
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(passwordSignInQuery().queryKey, organization(true), {
    updatedAt: Date.now() - agedBy,
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { reads, hook: renderHook(() => usePasswordSignIn(), { wrapper }) };
}

describe("usePasswordSignIn follows the centre's setting within the minute (QC-W-10)", () => {
  it("asks again when its answer is a minute old, bypassing the browser's five-minute cache", async () => {
    const { reads, hook } = setup(60_000);
    await waitFor(() => expect(hook.result.current).toBe("off"));
    expect(reads).toEqual(["no-cache"]);
  });

  it("keeps a fresh answer without asking", async () => {
    const { reads, hook } = setup(5_000);
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(hook.result.current).toBe("on");
    expect(reads).toEqual([]);
  });
});

describe("usePasswordSignIn when the answer fails (QC-W-9)", () => {
  it("is on at once and asks only once, though the app's own client would retry", async () => {
    let asked = 0;
    server.use(
      http.get(`${BASE_URL}/public/organization`, () => {
        asked += 1;
        return HttpResponse.json({ error: { code: "INTERNAL" } }, { status: 500 });
      }),
    );
    const client = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const hook = renderHook(() => usePasswordSignIn(), { wrapper });

    expect(hook.result.current).toBe("pending");
    await waitFor(() => expect(hook.result.current).toBe("on"), { timeout: 800 });
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(asked).toBe(1);
  });
});
