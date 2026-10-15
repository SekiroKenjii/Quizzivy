import { afterEach, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";
import { useLogout } from "@/features/auth/useSession";
import { heldSession, holdSession } from "@/features/take-test/heldSession";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { studentUser } from "@tests/support/fixtures";

vi.mock("@/lib/drafts/store", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/drafts/store")>()),
  clearAuthoringDrafts: async () => undefined,
}));

const BASE = "http://localhost:8080";
const SESSION = "018f0000-0000-7000-8000-00000000a001";

afterEach(() => sessionStorage.clear());

it("forgets the session the tab held for an attempt when the user signs out", async () => {
  useAuthStore.getState().setSession("t", studentUser);
  server.use(
    http.post(`${BASE}/auth/logout`, () => new HttpResponse(null, { status: 204 })),
  );
  holdSession("att-1", SESSION);
  expect(heldSession("att-1")).toBe(SESSION);
  const { result } = renderHook(() => useLogout(), {
    wrapper: ({ children }: Readonly<{ children: ReactNode }>) => (
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>{children}</MemoryRouter>
      </QueryClientProvider>
    ),
  });

  await act(() => result.current());

  expect(heldSession("att-1")).toBeNull();
  expect(sessionStorage.getItem("quizzivy.session.att-1")).toBeNull();
});
