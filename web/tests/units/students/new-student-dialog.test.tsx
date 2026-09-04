import { beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { useState } from "react";
import { NewStudentDialog } from "@/features/students/components/NewStudentDialog";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

const CREATED = {
  id: "018f0000-0000-7000-8000-0000000000e9",
  email: "trang@example.com",
  fullName: "Lê Thu Trang",
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: true,
  createdAt: "2026-01-01T00:00:00Z",
  disabledAt: null,
  classes: [],
  stats: {
    submittedCount: 0,
    flaggedCount: 0,
    activity: { live: false, lastAttemptAt: null },
  },
};

beforeEach(() => {
  server.use(
    http.get(`${BASE}/admin/classes`, () =>
      contractJson("/admin/classes", "get", 200, {
        page: 1,
        pageSize: 50,
        total: 0,
        items: [],
      }),
    ),
    http.post(`${BASE}/admin/students`, () =>
      contractJson("/admin/students", "post", 201, {
        user: CREATED,
        temporaryPassword: "tho-vang-42",
      }),
    ),
  );
});

/** Mirrors the page: the dialog's `open` is controlled from outside. */
function Harness() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        mở
      </button>
      <NewStudentDialog open={open} onOpenChange={setOpen} />
    </>
  );
}

function renderHarness() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Harness />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("adding a student", () => {
  it("does not reopen showing the last student's password", async () => {
    const user = renderHarness();

    await user.click(screen.getByRole("button", { name: "mở" }));
    await user.type(screen.getByLabelText("Họ và tên"), "Lê Thu Trang");
    await user.type(screen.getByLabelText("Email"), "trang@example.com");
    await user.click(screen.getByRole("button", { name: "Tạo tài khoản" }));

    expect(await screen.findByText("tho-vang-42")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Xong" }));

    await user.click(screen.getByRole("button", { name: "mở" }));
    expect(await screen.findByLabelText("Họ và tên")).toHaveValue("");
    expect(screen.queryByText("tho-vang-42")).toBeNull();
  });

  it("clears a half-typed form when cancelled", async () => {
    const user = renderHarness();

    await user.click(screen.getByRole("button", { name: "mở" }));
    await user.type(screen.getByLabelText("Họ và tên"), "Nhập nhầm");
    await user.click(screen.getByRole("button", { name: "Huỷ" }));

    await user.click(screen.getByRole("button", { name: "mở" }));
    expect(await screen.findByLabelText("Họ và tên")).toHaveValue("");
  });
});

/** The password is returned once; a stray Esc must not be the way it is lost. */
describe("while the temporary password is shown", () => {
  it("ignores Escape and closes only through Xong", async () => {
    const user = renderHarness();

    await user.click(screen.getByRole("button", { name: "mở" }));
    await user.type(screen.getByLabelText("Họ và tên"), "Lê Thu Trang");
    await user.type(screen.getByLabelText("Email"), "trang@example.com");
    await user.click(screen.getByRole("button", { name: "Tạo tài khoản" }));
    expect(await screen.findByText("tho-vang-42")).toBeInTheDocument();

    await user.keyboard("{Escape}");
    expect(screen.getByText("tho-vang-42")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Đóng" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "Xong" }));
    expect(screen.queryByText("tho-vang-42")).toBeNull();
  });
});
