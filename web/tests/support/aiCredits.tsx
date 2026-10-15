import { render } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import AiCreditsPage from "@/features/ai-credits/pages/admin/AiCreditsPage";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "./contractResponse";
import { adminUser } from "./fixtures";
import { server } from "./server";

globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

export type Credits = components["schemas"]["AiAdminCredits"];
export type Teacher = components["schemas"]["AiTeacherMonth"];
export type Pending = components["schemas"]["AiPendingRequest"];

export const BASE = "http://localhost:8080";

export const GIAU = "019535d9-3df7-79fb-b466-fa907fa17fa1";
export const DUC = "019535d9-3df7-79fb-b466-fa907fa17fa2";
export const BAO = "019535d9-3df7-79fb-b466-fa907fa17fa3";
export const TRANG = "019535d9-3df7-79fb-b466-fa907fa17fa4";
export const KHAI = "019535d9-3df7-79fb-b466-fa907fa17fa5";
export const LINH = "019535d9-3df7-79fb-b466-fa907fa17fa6";
export const REQUEST = "019535d9-3df7-79fb-b466-fa907fa17fc1";

function teacher(
  userId: string,
  name: string,
  email: string,
  over: Partial<Teacher>,
): Teacher {
  return {
    userId,
    name,
    email,
    allowance: 50000,
    granted: 0,
    used: 0,
    reserved: 0,
    custom: false,
    paused: false,
    status: "on_track",
    ...over,
  };
}

export const giau = teacher(GIAU, "Giàu Bùi", "giau.bui@quizzivy.com", {
  used: 18420,
});
export const duc = teacher(DUC, "Phan Minh Đức", "duc.phan@quizzivy.com", {
  used: 41300,
  status: "near_limit",
});
export const bao = teacher(BAO, "Đinh Quốc Bảo", "bao.dinh@quizzivy.com", {
  used: 30100,
  allowance: 80000,
  custom: true,
});
export const trang = teacher(TRANG, "Ngô Thu Trang", "trang.ngo@quizzivy.com", {
  used: 12880,
});
export const khai = teacher(KHAI, "Trương Văn Khải", "khai.truong@quizzivy.com", {
  used: 20000,
  allowance: 20000,
  custom: true,
  status: "at_limit",
});
export const linh = teacher(LINH, "Hà Mỹ Linh", "linh.ha@quizzivy.com", {
  used: 2140,
  paused: true,
  status: "paused",
});

export const ducRequest: Pending = {
  id: REQUEST,
  userId: DUC,
  teacherName: "Phan Minh Đức",
  month: "2026-09-01",
  amount: 20000,
  reason: "Chấm bài Writing Task 2 cho hai lớp trong tuần này.",
  createdAt: new Date(Date.now() - 2 * 3_600_000).toISOString(),
  used: 41300,
  allowance: 50000,
};

/**
 * credits is the Admin's September: 124,840 of a 200,000 pool used on the
 * 24th of 30 days, six teachers and one request waiting.
 */
export function credits(over: Partial<Credits> = {}): Credits {
  return {
    month: "2026-09-01",
    pool: {
      plan: 200000,
      added: 0,
      used: 124840,
      reserved: 0,
      total: 200000,
      left: 75160,
    },
    daysToGo: 6,
    onPace: 156050,
    withinPool: true,
    allowances: 300000,
    active: 6,
    idle: 3,
    teachers: [giau, duc, bao, trang, khai, linh],
    requests: [ducRequest],
    rules: {
      poolCredits: 200000,
      defaultAllowance: 50000,
      warnPercent: 80,
      onTeacherLimit: "request",
    },
    ...over,
  };
}

/** serveCredits answers the month read with `view`, counting how often it is asked. */
export function serveCredits(view: Credits = credits()) {
  const seen = { reads: 0, view };
  server.use(
    http.get(`${BASE}/admin/ai/credits`, () => {
      seen.reads += 1;
      return contractJson("/admin/ai/credits", "get", 200, seen.view);
    }),
  );
  return seen;
}

/** serveOff answers the month read with `501 AI_OFF`, as a server without a provider does. */
export function serveOff() {
  const seen = { reads: 0 };
  server.use(
    http.get(`${BASE}/admin/ai/credits`, () => {
      seen.reads += 1;
      return HttpResponse.json(
        {
          error: {
            code: "AI_OFF",
            message: "AI chưa được bật.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        },
        { status: 501 },
      );
    }),
  );
  return seen;
}

/** renderCreditsPage signs `user` in and renders the AI credits page with a toaster. */
export function renderCreditsPage(user = adminUser) {
  useAuthStore.getState().setSession("token", user);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [{ path: "/admin/ai", element: <AiCreditsPage /> }],
    { initialEntries: ["/admin/ai"] },
  );
  render(
    <QueryClientProvider client={client}>
      <Toaster />
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { client, user: userEvent.setup({ delay: null }) };
}
