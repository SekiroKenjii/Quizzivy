import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import ChangePasswordPage from "@/features/auth/pages/ChangePasswordPage";
import { PasswordSection } from "@/features/auth/components/SettingsSections";
import { saveJoinContext } from "@/features/join/context";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

const USER = {
  id: "018f0000-0000-7000-8000-0000000000a2",
  email: "an@example.com",
  fullName: "Nguyễn Văn An",
  role: "student" as const,
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: true,
  createdAt: "2026-01-01T00:00:00Z",
};

let bodies: unknown[] = [];

function changeAnswers(status: number, body?: unknown) {
  server.use(
    http.post(`${BASE}/auth/change-password`, async ({ request }) => {
      bodies.push(await request.json());
      if (status === 204) return new Response(null, { status: 204 });
      return contractJson("/auth/change-password", "post", status, body);
    }),
    http.get(`${BASE}/auth/me`, () =>
      contractJson("/auth/me", "get", 200, { ...USER, mustChangePassword: false }),
    ),
  );
}

function renderPage(entry = "/change-password") {
  const router = createMemoryRouter(
    [
      { path: "/change-password", element: <ChangePasswordPage /> },
      { path: "/app", element: <p>student home</p> },
      { path: "/app/tests", element: <p>tests</p> },
      { path: "/join/:code", element: <p>join page</p> },
    ],
    { initialEntries: [entry] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

const newField = () => screen.getByLabelText("Mật khẩu mới");
const againField = () => screen.getByLabelText("Nhập lại mật khẩu");
const save = () => screen.getByRole("button", { name: "Lưu và tiếp tục" });
const filledBars = () =>
  screen.getByTestId("password-meter").querySelectorAll("[data-filled]");
const rule = (name: string) =>
  within(screen.getByRole("list")).getByText(name).closest("li");

beforeEach(() => {
  bodies = [];
  sessionStorage.clear();
  useAuthStore.getState().setSession("token", USER);
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("the forced password change", () => {
  it("asks only for the new password, twice", () => {
    renderPage();
    expect(
      screen.getByRole("heading", { level: 1, name: "Chọn mật khẩu của bạn" }),
    ).toBeVisible();
    expect(newField()).toHaveAttribute("autocomplete", "new-password");
    expect(againField()).toHaveAttribute("autocomplete", "new-password");
    expect(screen.queryByLabelText("Mật khẩu hiện tại")).toBeNull();
  });

  it.each([
    ["", 0, null],
    ["abc", 1, "bg-danger"],
    ["abcdefgh", 2, "bg-warning"],
    ["abcdefg1", 3, "bg-success"],
    ["abcdefghijk1", 4, "bg-success"],
  ])("scores %j as %i bars", async (password, bars, tone) => {
    const user = userEvent.setup();
    renderPage();
    if (password) await user.type(newField(), password);
    expect(filledBars()).toHaveLength(bars);
    for (const bar of filledBars()) expect(bar).toHaveClass(tone ?? "");
  });

  it("ticks each rule as it is met, and states the one only the server checks", async () => {
    const user = userEvent.setup();
    renderPage();
    expect(rule("Ít nhất 8 ký tự")).toHaveAttribute("data-state", "unmet");
    expect(rule("Có số hoặc ký hiệu")).toHaveAttribute("data-state", "unmet");
    await user.type(newField(), "mật khẩu!");
    expect(rule("Ít nhất 8 ký tự")).toHaveAttribute("data-state", "met");
    expect(rule("Có số hoặc ký hiệu")).toHaveAttribute("data-state", "met");
    expect(rule("Khác mật khẩu tạm")).toHaveAttribute("data-state", "stated");
  });

  it("says when the two passwords differ", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.type(newField(), "matkhau1");
    await user.type(againField(), "matkhau2");
    expect(screen.getByText("Hai mật khẩu không giống nhau.")).toBeVisible();
    expect(againField()).toHaveAttribute("aria-invalid", "true");
  });

  it("holds the button until the rules are met, and says why when pressed", async () => {
    changeAnswers(204);
    const user = userEvent.setup();
    renderPage();
    expect(save()).toHaveAttribute("aria-disabled", "true");
    expect(save()).toBeEnabled();

    await user.type(newField(), "matkhau");
    await user.click(save());
    expect(rule("Có số hoặc ký hiệu")).toHaveAttribute("data-state", "failed");
    expect(newField()).toHaveAttribute("aria-invalid", "true");
    expect(newField()).toHaveFocus();
    expect(bodies).toEqual([]);

    await user.type(newField(), "1");
    await user.click(save());
    expect(screen.getByText("Hai mật khẩu không giống nhau.")).toBeVisible();
    expect(againField()).toHaveFocus();
    expect(bodies).toEqual([]);
  });

  it("sends no current password and goes on once it is saved", async () => {
    changeAnswers(204);
    const user = userEvent.setup();
    const router = renderPage("/change-password?next=/app/tests");
    await user.type(newField(), "matkhau1");
    await user.type(againField(), "matkhau1");
    expect(save()).not.toHaveAttribute("aria-disabled");
    await user.click(save());

    expect(await screen.findByText("tests")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/app/tests");
    expect(bodies).toEqual([{ newPassword: "matkhau1" }]);
    expect(useAuthStore.getState().user?.mustChangePassword).toBe(false);
  });

  it("goes back to finish a join once the password is changed", async () => {
    saveJoinContext({
      code: "K7QM2PXA",
      className: "TOEIC 600 Weekend",
      teacherName: "Hoàng Thương",
    });
    changeAnswers(204);
    const user = userEvent.setup();
    const router = renderPage();
    await user.type(newField(), "một-mật-khẩu-mới-9");
    await user.type(againField(), "một-mật-khẩu-mới-9");
    await user.click(save());

    expect(await screen.findByText("join page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/join/K7QM2PXA");
  });

  it("turns the stated rule into the server's refusal when the password is the temporary one", async () => {
    changeAnswers(400, {
      error: {
        code: "PASSWORD_UNCHANGED",
        message: "Mật khẩu mới phải khác mật khẩu hiện tại.",
        requestId: REQUEST_ID,
      },
    });
    const user = userEvent.setup();
    renderPage();
    await user.type(newField(), "song-tien-42");
    await user.type(againField(), "song-tien-42");
    await user.click(save());

    const refused = await screen.findByText(
      "Mật khẩu mới phải khác mật khẩu hiện tại.",
    );
    expect(refused.closest("li")).toHaveAttribute("data-state", "failed");
    expect(screen.queryByText("Khác mật khẩu tạm")).toBeNull();
    expect(newField()).toHaveAttribute("aria-invalid", "true");

    await user.type(newField(), "x");
    expect(screen.getByText("Khác mật khẩu tạm")).toBeVisible();
  });

  it("shows any other failure as the server wrote it", async () => {
    changeAnswers(400, {
      error: {
        code: "VALIDATION_FAILED",
        message: "Mật khẩu mới không hợp lệ.",
        requestId: REQUEST_ID,
      },
    });
    const user = userEvent.setup();
    renderPage();
    await user.type(newField(), "matkhau1");
    await user.type(againField(), "matkhau1");
    await user.click(save());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Mật khẩu mới không hợp lệ.",
    );
  });
});

describe("the settings password form", () => {
  it("refuses a short password at the field, without a request", async () => {
    changeAnswers(204);
    const user = userEvent.setup();
    render(<PasswordSection />);

    const field = screen.getByLabelText("Mật khẩu mới");
    await user.type(field, "abc");
    await user.click(screen.getByRole("button", { name: "Đổi mật khẩu" }));

    expect(
      await screen.findByText("Mật khẩu mới cần ít nhất 8 ký tự."),
    ).toBeInTheDocument();
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(bodies).toEqual([]);
  });

  it("refuses a password of letters only, in the same words as the forced change", async () => {
    changeAnswers(204);
    const user = userEvent.setup();
    render(<PasswordSection />);

    await user.type(screen.getByLabelText("Mật khẩu mới"), "mậtkhẩuđẹp");
    await user.click(screen.getByRole("button", { name: "Đổi mật khẩu" }));

    expect(
      await screen.findByText("Mật khẩu mới cần có số hoặc ký hiệu."),
    ).toBeInTheDocument();
    expect(bodies).toEqual([]);
  });

  it("explains a refusal to reuse the current password", async () => {
    changeAnswers(400, {
      error: {
        code: "PASSWORD_UNCHANGED",
        message: "Mật khẩu mới phải khác mật khẩu hiện tại.",
        requestId: REQUEST_ID,
      },
    });
    useAuthStore.getState().setSession("token", { ...USER, mustChangePassword: false });
    const user = userEvent.setup();
    render(<PasswordSection />);

    await user.type(screen.getByLabelText("Mật khẩu hiện tại"), "matkhau1");
    await user.type(screen.getByLabelText("Mật khẩu mới"), "matkhau1");
    await user.click(screen.getByRole("button", { name: "Đổi mật khẩu" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Mật khẩu mới phải khác mật khẩu hiện tại.",
    );
  });
});
