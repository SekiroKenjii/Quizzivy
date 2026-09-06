import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import StudentsListPage from "@/features/students/pages/StudentsListPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const HAN = "018f0000-0000-7000-8000-0000000000e1";

const han = {
  id: HAN,
  email: "han.pham@gmail.com",
  fullName: "Phạm Gia Hân",
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
  disabledAt: null,
  classes: [],
  stats: {
    submittedCount: 0,
    flaggedCount: 0,
    activity: { live: false, lastAttemptAt: null },
  },
};

let patches: unknown[] = [];

beforeEach(() => {
  patches = [];
  server.use(
    http.get(`${BASE}/admin/students`, () =>
      contractJson("/admin/students", "get", 200, {
        items: [han],
        page: 1,
        pageSize: 20,
        total: 1,
        facets: { total: 1, activeLast7Days: 0 },
      }),
    ),
    http.get(`${BASE}/admin/students/${HAN}`, () =>
      contractJson("/admin/students/{id}", "get", 200, han),
    ),
    http.patch(`${BASE}/admin/students/${HAN}`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      patches.push(body);
      return contractJson("/admin/students/{id}", "patch", 200, { ...han, ...body });
    }),
  );
});

function renderList() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/students", element: <StudentsListPage /> }],
    { initialEntries: ["/admin/students"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("G-07a: editing a student in the drawer", () => {
  it("turns the header into two fields and writes only on Lưu", async () => {
    const user = renderList();
    await user.click(await screen.findByRole("button", { name: /Phạm Gia Hân/ }));
    const drawer = await screen.findByRole("complementary");
    await user.click(within(drawer).getByRole("button", { name: "Sửa thông tin" }));

    expect(within(drawer).getByText(/Email cũng là tên đăng nhập/)).toBeInTheDocument();
    const email = within(drawer).getByLabelText("Email");
    await user.clear(email);
    await user.type(email, "han@example.com");
    expect(patches).toHaveLength(0);

    await user.click(within(drawer).getByRole("button", { name: "Lưu" }));
    await waitFor(() =>
      expect(patches).toEqual([{ fullName: "Phạm Gia Hân", email: "han@example.com" }]),
    );
  });

  it("refuses an email that is not one, before asking the server", async () => {
    const user = renderList();
    await user.click(await screen.findByRole("button", { name: /Phạm Gia Hân/ }));
    const drawer = await screen.findByRole("complementary");
    await user.click(within(drawer).getByRole("button", { name: "Sửa thông tin" }));

    const email = within(drawer).getByLabelText("Email");
    await user.clear(email);
    await user.type(email, "not-an-email");
    await user.click(within(drawer).getByRole("button", { name: "Lưu" }));

    expect(
      await within(drawer).findByText("Email chưa đúng định dạng."),
    ).toBeInTheDocument();
    expect(patches).toHaveLength(0);
  });
});
