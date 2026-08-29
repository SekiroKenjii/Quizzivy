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
const DUNG = "018f0000-0000-7000-8000-0000000000e2";

/**
 * A temporary password is shown once and stored only as a hash. Every test here
 * is about that: it must not follow the teacher to another student, and it must
 * not be destroyed by something as ordinary as typing in the search box.
 */
function student(id: string, fullName: string, email: string) {
  return {
    id,
    email,
    fullName,
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
}

const HAN_ROW = student(HAN, "Phạm Gia Hân", "han@example.com");
const DUNG_ROW = student(DUNG, "Hoàng Tiến Dũng", "dung@example.com");

beforeEach(() => {
  server.use(
    http.get(`${BASE}/admin/students`, ({ request }) => {
      const q = new URL(request.url).searchParams.get("q") ?? "";
      const all = [HAN_ROW, DUNG_ROW];
      const items = q === "" ? all : all.filter((s) => s.fullName.includes(q));
      return contractJson("/admin/students", "get", 200, {
        items,
        nextCursor: null,
        facets: { total: all.length, activeLast7Days: 0 },
      });
    }),
    http.get(`${BASE}/admin/students/:id`, ({ params }) =>
      contractJson(
        "/admin/students/{id}",
        "get",
        200,
        params["id"] === HAN ? HAN_ROW : DUNG_ROW,
      ),
    ),
    http.post(`${BASE}/admin/students/:id/reset-password`, ({ params }) =>
      contractJson("/admin/students/{id}/reset-password", "post", 200, {
        temporaryPassword: params["id"] === HAN ? "tho-vang-42" : "cay-dua-13",
      }),
    ),
  );
});

function renderPage() {
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

async function openDrawerFor(user: ReturnType<typeof userEvent.setup>, name: string) {
  const table = await screen.findByRole("table");
  await user.click(within(table).getByText(name));
}

describe("the one-time password in the student drawer", () => {
  it("does not follow the teacher to the next student", async () => {
    const user = renderPage();

    await openDrawerFor(user, "Phạm Gia Hân");
    await user.click(await screen.findByRole("button", { name: "Đặt mật khẩu tạm" }));
    expect(await screen.findByText("tho-vang-42")).toBeInTheDocument();

    // Clicking another row never passes through "nothing selected", so without
    // an identity the panel keeps its state and prints Hân's password under
    // Dũng's name.
    await openDrawerFor(user, "Hoàng Tiến Dũng");

    await screen.findByRole("complementary", { name: /Hoàng Tiến Dũng/ });
    expect(screen.queryByText("tho-vang-42")).toBeNull();
  });

  it("survives a search that filters the table underneath it", async () => {
    const user = renderPage();

    await openDrawerFor(user, "Hoàng Tiến Dũng");
    await user.click(await screen.findByRole("button", { name: "Đặt mật khẩu tạm" }));
    expect(await screen.findByText("cay-dua-13")).toBeInTheDocument();

    // The password exists nowhere else: the server keeps only a hash.
    await user.type(screen.getByLabelText("Tìm theo tên hoặc email"), "Hân");

    await waitFor(() =>
      expect(screen.queryByText("Hoàng Tiến Dũng", { selector: "td *" })).toBeNull(),
    );
    expect(screen.getByText("cay-dua-13")).toBeInTheDocument();
  });
});

/**
 * Disabling used to be a one-way door: the student vanished from every listing
 * and nothing in the API could name them again, so `updateStudent`'s
 * `disabled: false` was unreachable.
 */
describe("suspending and restoring a student", () => {
  it("finds suspended accounts only when asked, and offers to restore them", async () => {
    const suspended = {
      ...student(HAN, "Phạm Gia Hân", "han@example.com"),
      disabledAt: "2026-08-01T00:00:00Z",
    };
    server.use(
      http.get(`${BASE}/admin/students`, ({ request }) => {
        const status = new URL(request.url).searchParams.get("status");
        return contractJson("/admin/students", "get", 200, {
          items: status === "disabled" ? [suspended] : [DUNG_ROW],
          nextCursor: null,
          facets: { total: 1, activeLast7Days: 0 },
        });
      }),
      http.get(`${BASE}/admin/students/:id`, () =>
        contractJson("/admin/students/{id}", "get", 200, suspended),
      ),
    );
    const user = renderPage();

    // Not in the default listing.
    const table = await screen.findByRole("table");
    expect(within(table).queryByText("Phạm Gia Hân")).toBeNull();

    await user.click(screen.getByLabelText("Tài khoản đã khoá"));

    await waitFor(() =>
      expect(
        within(screen.getByRole("table")).getByText("Phạm Gia Hân"),
      ).toBeInTheDocument(),
    );

    await user.click(within(screen.getByRole("table")).getByText("Phạm Gia Hân"));
    const panel = await screen.findByRole("complementary", { name: /Phạm Gia Hân/ });
    expect(within(panel).getByText("đã khoá")).toBeInTheDocument();
    // The way back that did not exist before.
    expect(within(panel).getByRole("button", { name: "Mở khoá" })).toBeInTheDocument();
  });
});
