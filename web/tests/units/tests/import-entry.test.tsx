import { beforeEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestsListPage from "@/features/tests/pages/TestsListPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

let checks = 0;
let rendered: QueryClient;

function serve(intakeEnabled: boolean, processingEnabled: boolean) {
  server.use(
    http.get(`${BASE}/admin/imports/capabilities`, () => {
      checks += 1;
      return contractJson("/admin/imports/capabilities", "get", 200, {
        intakeEnabled,
        processingEnabled,
      });
    }),
  );
}

beforeEach(() => {
  checks = 0;
  server.use(
    http.get(`${BASE}/admin/tests`, () =>
      contractJson("/admin/tests", "get", 200, {
        items: [],
        page: 1,
        pageSize: 50,
        total: 0,
        facets: { all: 0, draft: 0, published: 0, archived: 0 },
        tags: [],
      }),
    ),
  );
});

function renderList() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  rendered = client;
  const router = createMemoryRouter(
    [{ path: "/admin/tests", element: <TestsListPage /> }],
    { initialEntries: ["/admin/tests"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

const history = () => screen.queryByRole("link", { name: "Lịch sử nhập" });
const importWord = () => screen.queryByRole("link", { name: "Nhập đề từ Word" });

describe("the tests list's way into Word import", () => {
  it("is absent where the server has no import storage", async () => {
    serve(false, false);
    renderList();

    await screen.findByText("Chưa có đề thi nào.");
    await waitFor(() =>
      expect(rendered.getQueryState(["word-import-capabilities"])?.status).toBe(
        "success",
      ),
    );
    await act(() => new Promise((resolve) => setTimeout(resolve, 20)));
    expect(checks).toBe(1);
    expect(history()).toBeNull();
    expect(importWord()).toBeNull();
  });

  it("keeps only the history where no worker processes imports", async () => {
    serve(true, false);
    renderList();

    expect(await screen.findByRole("link", { name: "Lịch sử nhập" })).toHaveAttribute(
      "href",
      "/admin/imports",
    );
    expect(importWord()).toBeNull();
  });

  it("offers both where import runs end to end", async () => {
    serve(true, true);
    renderList();

    expect(
      await screen.findByRole("link", { name: "Nhập đề từ Word" }),
    ).toHaveAttribute("href", "/admin/imports/new");
    expect(history()).toBeInTheDocument();
  });
});
