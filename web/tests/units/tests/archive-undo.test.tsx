import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestsListPage from "@/features/tests/pages/TestsListPage";
import { Toaster } from "@/components/ui/sonner";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const SAVED_AT = "2026-01-01T00:00:00Z";
const ARCHIVED_AT = "2026-02-02T02:02:02Z";

function test(over: Record<string, unknown> = {}) {
  return {
    id: TEST_ID,
    title: "Unit 5",
    description: null,
    status: "published" as const,
    currentVersion: 1,
    totalPoints: 10,
    questionCount: 4,
    audioCount: 0,
    sections: [],
    createdAt: SAVED_AT,
    updatedAt: SAVED_AT,
    ...over,
  };
}

let patches: Record<string, unknown>[] = [];
let archived = false;

beforeEach(() => {
  patches = [];
  archived = false;
  server.use(
    http.get(`${BASE}/admin/tests`, () =>
      contractJson("/admin/tests", "get", 200, {
        items: [
          archived ? test({ status: "archived", updatedAt: ARCHIVED_AT }) : test(),
        ],
        page: 1,
        pageSize: 50,
        total: 1,
        facets: {
          all: 1,
          draft: 0,
          published: archived ? 0 : 1,
          archived: archived ? 1 : 0,
        },
        tags: [],
      }),
    ),
    http.patch(`${BASE}/admin/tests/:id`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      patches.push(body);
      archived = body["status"] === "archived";
      return contractJson("/admin/tests/{id}", "patch", 200, {
        ...test({ status: body["status"], updatedAt: ARCHIVED_AT }),
      });
    }),
  );
});

function renderList() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/admin/tests", element: <TestsListPage /> },
      { path: "/admin/tests/:id/edit", element: <p>builder</p> },
    ],
    { initialEntries: ["/admin/tests"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("undoing an archive from the toast", () => {
  it("guards on the row the server just wrote, and puts the status back", async () => {
    const user = renderList();
    await user.click(await screen.findByRole("button", { name: /Thao tác/ }));
    await user.click(await screen.findByRole("menuitem", { name: "Lưu trữ" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Lưu trữ" }));

    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]).toEqual({ expectedUpdatedAt: SAVED_AT, status: "archived" });

    await user.click(
      await screen.findByRole("button", { name: "Hoàn tác" }, { timeout: 5000 }),
    );

    await waitFor(() => expect(patches).toHaveLength(2));
    expect(patches[1]).toEqual({
      expectedUpdatedAt: ARCHIVED_AT,
      status: "published",
    });
  });
});
