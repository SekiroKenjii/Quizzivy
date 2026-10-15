import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import AuditLogPage from "@/features/audit/pages/admin/AuditLogPage";
import type { AuditEntry, AuditFacets } from "@/features/audit/api";
import { BASE_URL } from "@/lib/api/client";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";

/** REQUEST_ID is a request id the error envelope accepts. */
export const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

/** ACTOR is the account most entries are by. */
export const ACTOR = { id: "019535d9-3df7-79fb-b466-fa907fa17f01", name: "Giàu Bùi" };

/** entry builds an entry of the log: a class created at 20:31 on 24 September 2026, Vietnam time. */
export function entry(over: Partial<AuditEntry> & { id: number }): AuditEntry {
  return {
    occurredAt: "2026-09-24T13:31:00Z",
    actor: ACTOR,
    action: "class.created",
    category: "classes",
    entity: {
      type: "class",
      id: "019535d9-3df7-79fb-b466-fa907fa17f10",
      label: "IELTS 6.5 Evening",
    },
    ip: "203.0.113.8",
    device: "Mac · Chrome",
    changes: [{ field: "code", from: "H2LV-5QNC", to: "K7QM-2PXA" }],
    details: null,
    ...over,
  };
}

/** FACETS are the counts a screen shows unless a test says otherwise. */
export const FACETS: AuditFacets = {
  all: 1248,
  accounts: 312,
  classes: 140,
  attempts: 410,
  tests: 301,
  media: 0,
  imports: 7,
};

type Served = Readonly<{
  items: readonly AuditEntry[];
  total?: number;
  page?: number;
  facets?: AuditFacets;
}>;

/**
 * serves answers `GET /admin/audit-log` with `answer(url)` and returns every
 * address the screen asked, in order.
 */
export function serves(answer: (url: URL) => Served) {
  const asked: URL[] = [];
  server.use(
    http.get(`${BASE_URL}/admin/audit-log`, ({ request }) => {
      const url = new URL(request.url);
      asked.push(url);
      const body = answer(url);
      return contractJson("/admin/audit-log", "get", 200, {
        items: [...body.items],
        page: body.page ?? Number(url.searchParams.get("page") ?? 1),
        pageSize: 10,
        total: body.total ?? body.items.length,
        facets: body.facets ?? FACETS,
      });
    }),
  );
  return asked;
}

/** lastAsked is the newest address the screen asked. */
export function lastAsked(asked: readonly URL[]): URL {
  return asked[asked.length - 1]!;
}

/** renderAudit renders the screen at `/admin/audit` with the address's `search`, with a toaster. */
export function renderAudit(search = "") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/audit", element: <AuditLogPage /> }],
    { initialEntries: [`/admin/audit${search}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { router, client };
}
