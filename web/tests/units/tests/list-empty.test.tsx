import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestsListPage from "@/features/tests/pages/TestsListPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";

/**
 * The empty tests list, which is what a fresh database shows and therefore what
 * CI meets on every run. E2E 1a passed locally for weeks against a database that
 * had rows in it and failed the first time it ran against an empty one.
 */
const created = {
  id: TEST_ID,
  title: "Đề thi chưa đặt tên",
  description: null,
  status: "draft" as const,
  currentVersion: 0,
  totalPoints: 1,
  questionCount: 0,
  sections: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

let creations = 0;

beforeEach(() => {
  creations = 0;
  server.use(
    http.get(`${BASE}/admin/tests`, () =>
      contractJson("/admin/tests", "get", 200, { items: [], nextCursor: null }),
    ),
    http.post(`${BASE}/admin/tests`, () => {
      creations += 1;
      return contractJson("/admin/tests", "post", 201, created);
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
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("the tests list with nothing in it", () => {
  it("says the bank is empty rather than showing a table with no rows", async () => {
    renderList();

    expect(await screen.findByText("Chưa có đề thi nào.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("offers the create action twice, and the page header's comes first", async () => {
    renderList();
    await screen.findByText("Chưa có đề thi nào.");

    // Two controls share one accessible name here, which is why the E2E has to
    // disambiguate. DOM order is the thing it relies on, so it is pinned.
    const buttons = screen.getAllByRole("button", { name: "Đề thi mới" });
    expect(buttons).toHaveLength(2);
    expect(buttons[0]!.compareDocumentPosition(buttons[1]!)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
  });

  it("creates a test from the header button", async () => {
    const user = renderList();
    await screen.findByText("Chưa có đề thi nào.");

    await user.click(screen.getAllByRole("button", { name: "Đề thi mới" })[0]!);

    await waitFor(() => expect(creations).toBe(1));
    expect(await screen.findByText("builder")).toBeInTheDocument();
  });

  it("creates a test from the empty state too", async () => {
    const user = renderList();
    await screen.findByText("Chưa có đề thi nào.");

    await user.click(screen.getAllByRole("button", { name: "Đề thi mới" })[1]!);

    await waitFor(() => expect(creations).toBe(1));
  });

  it("distinguishes an empty database from an empty filter", async () => {
    const user = renderList();
    await screen.findByText("Chưa có đề thi nào.");

    await user.click(screen.getByRole("tab", { name: "Đã phát hành" }));

    expect(await screen.findByText("Không có đề thi nào khớp.")).toBeInTheDocument();
  });
});
