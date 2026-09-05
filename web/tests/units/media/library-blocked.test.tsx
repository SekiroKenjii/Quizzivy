import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import MediaLibraryPage from "@/features/media/pages/MediaLibraryPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000d1";

let deletes = 0;

beforeEach(() => {
  deletes = 0;
  server.use(
    http.get(`${BASE}/admin/media`, () =>
      contractJson("/admin/media", "get", 200, {
        items: [
          {
            id: "018f0000-0000-7000-8000-0000000000e1",
            kind: "audio",
            url: "https://example.test/a.mp3",
            mimeType: "audio/mpeg",
            bytes: 2_400_000,
            durationMs: 110_000,
            originalFilename: "unit5-listening-2.mp3",
            createdAt: "2026-08-26T00:00:00Z",
            usageCount: 2,
            usedIn: [
              {
                id: TEST_ID,
                title: "Unit 5 — Present perfect & listening",
                version: 3,
              },
              {
                id: "018f0000-0000-7000-8000-0000000000d2",
                title: "Listening practice 02",
                version: 1,
              },
            ],
          },
        ],
        page: 1,
        pageSize: 24,
        total: 1,
      }),
    ),
    http.delete(`${BASE}/admin/media/:id`, () => {
      deletes += 1;
      return new Response(null, { status: 204 });
    }),
  );
});

function renderLibrary() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/media", element: <MediaLibraryPage /> }],
    {
      initialEntries: ["/admin/media"],
    },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("deleting a file a published test uses", () => {
  it("names the tests holding it instead of failing after the click", async () => {
    const user = renderLibrary();

    await user.click(
      await screen.findByRole("button", { name: "Xoá unit5-listening-2.mp3" }),
    );

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Không xoá được tệp này")).toBeInTheDocument();
    const link = within(dialog).getByRole("link", {
      name: "Unit 5 — Present perfect & listening",
    });
    expect(link).toHaveAttribute("href", `/admin/tests/${TEST_ID}`);
    expect(within(dialog).getByText("v3")).toBeInTheDocument();
    expect(
      within(dialog).getByRole("link", { name: "Listening practice 02" }),
    ).toBeInTheDocument();
    expect(deletes).toBe(0);
  });
});
