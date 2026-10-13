import type { ReactNode } from "react";
import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import type { AiMonthUsage } from "@/features/ai/api";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import "@/lib/i18n";

/** BASE is the API origin the MSW handlers answer on. */
export const BASE = "http://localhost:8080";

/** month is a teacher's month of AI credits, with `patch` laid over the defaults. */
export function month(patch: Partial<AiMonthUsage> = {}): AiMonthUsage {
  return {
    month: "2026-10-01",
    used: 1200,
    allowance: 20000,
    left: 18800,
    paused: false,
    requests: true,
    pending: null,
    categories: [
      { category: "generation", credits: 1200 },
      { category: "grading", credits: 0 },
      { category: "explanations", credits: 0 },
      { category: "imports", credits: 0 },
      { category: "checks", credits: 0 },
    ],
    ...patch,
  };
}

/**
 * aiOn signs a teacher in and answers the status and the usage reads: the
 * assistant on unless `ai` is false, and the month `usage` gives.
 */
export function aiOn({
  ai = true,
  usage = month(),
}: { ai?: boolean; usage?: AiMonthUsage } = {}) {
  useAuthStore.setState({ accessToken: "token", user: teacherUser });
  server.use(
    http.get(`${BASE}/public/status`, () =>
      contractJson("/public/status", "get", 200, { maintenance: null, ai }),
    ),
    http.get(`${BASE}/teacher/ai/usage`, () =>
      contractJson("/teacher/ai/usage", "get", 200, usage),
    ),
  );
}

/** aiError answers an AI operation with the error envelope, and `Retry-After` on a 503. */
export function aiError(
  status: number,
  code: string,
  details?: Record<string, unknown>,
) {
  return HttpResponse.json(
    {
      error: { code, message: `server says ${code}`, ...(details ? { details } : {}) },
    },
    { status, headers: status === 503 ? { "Retry-After": "30" } : {} },
  );
}

/** renderWithApp renders `ui` inside a query client, a router and the toaster, with a user-event session. */
export function renderWithApp(ui: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/", element: ui },
      { path: "*", element: <p>elsewhere</p> },
    ],
    { initialEntries: ["/"] },
  );
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { ...view, client, router, user: userEvent.setup() };
}
