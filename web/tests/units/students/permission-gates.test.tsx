import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import StudentsListPage from "@/features/students/pages/teacher/StudentsListPage";
import { useAuthStore } from "@/stores/auth";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const HAN = "018f0000-0000-7000-8000-0000000000e1";
const DUNG = "018f0000-0000-7000-8000-0000000000e2";

function student(
  id: string,
  fullName: string,
  email: string,
  disabledAt: string | null,
) {
  return {
    id,
    email,
    fullName,
    hasPassword: true,
    linkedProviders: [],
    mustChangePassword: false,
    createdAt: "2026-01-01T00:00:00Z",
    disabledAt,
    classes: [],
    stats: {
      submittedCount: 0,
      flaggedCount: 0,
      activity: { live: false, lastAttemptAt: null },
    },
  };
}

const ACTIVE = student(DUNG, "Hoàng Tiến Dũng", "dung@example.com", null);
const DISABLED = student(
  HAN,
  "Phạm Gia Hân",
  "han@example.com",
  "2026-08-01T00:00:00Z",
);

beforeEach(() => {
  server.use(
    http.get(`${BASE}/teacher/students`, ({ request }) => {
      const status = new URL(request.url).searchParams.get("status");
      return contractJson("/teacher/students", "get", 200, {
        items: status === "disabled" ? [DISABLED] : [ACTIVE],
        page: 1,
        pageSize: 50,
        total: 1,
        facets: { total: 1, activeLast7Days: 0 },
      });
    }),
    http.get(`${BASE}/teacher/students/:id`, ({ params }) =>
      contractJson(
        "/teacher/students/{id}",
        "get",
        200,
        params["id"] === HAN ? DISABLED : ACTIVE,
      ),
    ),
  );
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function renderPageAs(signedIn: components["schemas"]["CurrentUser"]) {
  useAuthStore.getState().setUser(signedIn);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/students", element: <StudentsListPage /> }],
    { initialEntries: ["/admin/students"] },
  );
  const { unmount } = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), unmount };
}

async function rowFor(name: string) {
  const table = await screen.findByRole("table");
  return within(table).findByRole("row", { name: new RegExp(name) });
}

async function openDrawerFor(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(within(await rowFor(name)).getByText(name));
  return screen.findByRole("complementary", { name: new RegExp(name) });
}

async function showDisabled(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByRole("table");
  await user.click(screen.getByLabelText("Tài khoản đã khoá"));
}

describe("the Students controls that need people.users.manage", () => {
  it("a Teacher's student record has no Disable control", async () => {
    const teacher = renderPageAs(teacherUser);
    const teacherPanel = await openDrawerFor(teacher.user, "Hoàng Tiến Dũng");
    expect(
      within(teacherPanel).getByRole("button", { name: "Đặt mật khẩu tạm" }),
    ).toBeInTheDocument();
    expect(
      within(teacherPanel).queryByRole("button", { name: "Khoá tài khoản" }),
    ).toBeNull();
    teacher.unmount();

    const admin = renderPageAs(adminUser);
    const adminPanel = await openDrawerFor(admin.user, "Hoàng Tiến Dũng");
    expect(
      within(adminPanel).getByRole("button", { name: "Khoá tài khoản" }),
    ).toBeInTheDocument();
  });

  it("a Teacher cannot restore a disabled account from the record", async () => {
    const teacher = renderPageAs(teacherUser);
    await showDisabled(teacher.user);
    const teacherPanel = await openDrawerFor(teacher.user, "Phạm Gia Hân");
    expect(within(teacherPanel).getByText("đã khoá")).toBeInTheDocument();
    expect(within(teacherPanel).queryByRole("button", { name: "Mở khoá" })).toBeNull();
    teacher.unmount();

    const admin = renderPageAs(adminUser);
    await showDisabled(admin.user);
    const adminPanel = await openDrawerFor(admin.user, "Phạm Gia Hân");
    expect(
      within(adminPanel).getByRole("button", { name: "Mở khoá" }),
    ).toBeInTheDocument();
  });

  it("a Teacher's student list offers no bulk actions", async () => {
    const teacher = renderPageAs(teacherUser);
    await rowFor("Hoàng Tiến Dũng");
    expect(within(screen.getByRole("table")).queryAllByRole("checkbox")).toHaveLength(
      0,
    );
    expect(
      screen.queryByRole("button", { name: "Vô hiệu hoá các mục đã chọn" }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: "Xoá vĩnh viễn" })).toBeNull();
    teacher.unmount();

    const admin = renderPageAs(adminUser);
    const adminRow = await rowFor("Hoàng Tiến Dũng");
    await admin.user.click(
      within(adminRow).getByRole("checkbox", { name: "Chọn Hoàng Tiến Dũng" }),
    );
    expect(
      screen.getByRole("button", { name: "Vô hiệu hoá các mục đã chọn" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Xoá vĩnh viễn" })).toBeInTheDocument();
  });

  it("a Teacher's list offers no permanent delete on a disabled student", async () => {
    const teacher = renderPageAs(teacherUser);
    await showDisabled(teacher.user);
    const teacherRow = await rowFor("Phạm Gia Hân");
    expect(
      within(teacherRow).queryByRole("button", { name: "Xoá vĩnh viễn Phạm Gia Hân" }),
    ).toBeNull();
    teacher.unmount();

    const admin = renderPageAs(adminUser);
    await showDisabled(admin.user);
    const adminRow = await rowFor("Phạm Gia Hân");
    expect(
      within(adminRow).getByRole("button", { name: "Xoá vĩnh viễn Phạm Gia Hân" }),
    ).toBeInTheDocument();
  });
});
