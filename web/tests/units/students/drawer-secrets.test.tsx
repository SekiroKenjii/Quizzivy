import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import { BASE, renderStudents, rowOf, serveStudents, student } from "./support";
import "@/lib/i18n";

const HAN = student("018f0000-0000-7000-8000-0000000000e1", "Phạm Gia Hân");
const DUNG = student("018f0000-0000-7000-8000-0000000000e2", "Hoàng Tiến Dũng");

afterEach(() => {
  useAuthStore.getState().clearSession();
});

async function openRecord(
  user: ReturnType<typeof renderStudents>["user"],
  name: string,
) {
  await user.click(within(await rowOf(name)).getByText(name));
  return screen.findByRole("dialog", { name });
}

/**
 * A temporary password is shown once and stored only as a hash. It must not
 * follow the teacher to another student's record.
 */
describe("the one-time password in a student's record", () => {
  beforeEach(() => {
    serveStudents([HAN, DUNG]);
    server.use(
      http.post(`${BASE}/teacher/students/:id/reset-password`, ({ params }) =>
        contractJson("/teacher/students/{id}/reset-password", "post", 200, {
          temporaryPassword:
            params["id"] === HAN.id ? "tho-vang-cay-42" : "cay-dua-mut-13",
        }),
      ),
    );
  });

  it("asks first, shows it once, and does not follow the teacher to the next student", async () => {
    const { user } = renderStudents(teacherUser);

    const record = await openRecord(user, "Phạm Gia Hân");
    await user.click(within(record).getByRole("button", { name: "Đặt mật khẩu tạm" }));
    await user.click(
      within(
        await screen.findByRole("dialog", { name: /Đặt lại mật khẩu cho/ }),
      ).getByRole("button", { name: "Đặt lại mật khẩu" }),
    );
    const sheet = await screen.findByRole("dialog", { name: /Mật khẩu tạm/ });
    expect(within(sheet).getByText("tho-vang-cay-42")).toBeVisible();
    await user.click(within(sheet).getByRole("button", { name: "Xong" }));
    await user.click(within(record).getByRole("button", { name: "Đóng" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    await openRecord(user, "Hoàng Tiến Dũng");
    expect(screen.queryByText("tho-vang-cay-42")).toBeNull();
  });
});

/**
 * Disabling used to be a one-way door: the student vanished from every listing
 * and nothing in the API could name them again.
 */
describe("suspending and restoring a student", () => {
  it("finds suspended accounts through the Status facet, and offers an admin the way back", async () => {
    const suspended = {
      ...HAN,
      disabledAt: "2026-08-01T00:00:00Z",
      accountStatus: "disabled" as const,
    };
    server.use(
      http.get(`${BASE}/teacher/students`, ({ request }) => {
        const status = new URL(request.url).searchParams.get("status");
        const items = status === "disabled" ? [suspended] : [DUNG];
        return contractJson("/teacher/students", "get", 200, {
          items,
          page: 1,
          pageSize: 20,
          total: items.length,
          facets: { total: 1, activeLast7Days: 0 },
        });
      }),
      http.get(`${BASE}/teacher/students/:id`, () =>
        contractJson("/teacher/students/{id}", "get", 200, suspended),
      ),
      http.get(`${BASE}/teacher/classes`, () =>
        contractJson("/teacher/classes", "get", 200, {
          items: [],
          page: 1,
          pageSize: 100,
          total: 0,
          facets: { all: 0, joinable: 0, archived: 0, students: 0 },
        }),
      ),
    );
    const { user } = renderStudents(adminUser, "/teacher/students?account=suspended");

    const row = await rowOf("Phạm Gia Hân");
    expect(row).toHaveTextContent("Đã khoá");
    const record = await openRecord(user, "Phạm Gia Hân");
    expect(within(record).getByText("đã khoá")).toBeInTheDocument();
    expect(within(record).getByRole("button", { name: "Mở khoá" })).toBeInTheDocument();
  });
});
