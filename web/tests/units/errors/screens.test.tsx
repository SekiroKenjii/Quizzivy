import { afterEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import { ErrorBoundary } from "@/app/ErrorBoundary";
import ForbiddenPage from "@/app/pages/ForbiddenPage";
import { MaintenancePage } from "@/app/pages/MaintenancePage";
import NotFoundPage from "@/app/pages/NotFoundPage";
import { BrandLockup, BrandMark } from "@/components/shared/Brand";
import { ApiError } from "@/lib/api/errors";
import { useAuthStore } from "@/stores/auth";
import { adminUser, studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

function renderAt(element: React.ReactElement, path = "/app/assignments/8f2c-unit-5") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const routes: RouteObject[] = [
    { path: "*", element },
    { path: "/login", element: <p>login page</p> },
    { path: "/app", element: <p>student home</p> },
    { path: "/admin", element: <p>admin home</p> },
  ];
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

function Throws({ error }: Readonly<{ error: unknown }>): never {
  throw error;
}

function renderFailure(error: unknown) {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  const router = createMemoryRouter(
    [{ path: "*", ErrorBoundary, element: <Throws error={error} /> }],
    { initialEntries: ["/app/tests"] },
  );
  render(<RouterProvider router={router} />);
}

afterEach(() => {
  useAuthStore.setState({ user: null });
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("the 404", () => {
  it("puts the address that failed on the page itself", () => {
    renderAt(<NotFoundPage />, "/app/assignments/8f2c-unit-5");
    expect(screen.getByText("/app/assignments/8f2c-unit-5")).toBeInTheDocument();
  });

  it("ranks home above back", () => {
    renderAt(<NotFoundPage />);
    const home = screen.getByText("Về trang chủ");
    const back = screen.getByText("Quay lại");
    expect(
      home.compareDocumentPosition(back) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("never prints the status number", () => {
    renderAt(<NotFoundPage />);
    expect(document.body.textContent).not.toMatch(/\b404\b/);
  });

  it.each([
    [null, "/login"],
    [studentUser, "/app"],
    [adminUser, "/admin"],
  ])("sends home to the caller's console (%#)", (user, home) => {
    useAuthStore.setState({ user });
    renderAt(<NotFoundPage />);
    expect(screen.getByRole("link", { name: "Về trang chủ" })).toHaveAttribute(
      "href",
      home,
    );
  });

  it("goes home rather than out of the app when there is nothing to go back to", async () => {
    useAuthStore.setState({ user: studentUser });
    const router = renderAt(<NotFoundPage />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Quay lại" }));
    expect(router.state.location.pathname).toBe("/app");
  });

  it("is also what a route answering 404 renders", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    const router = createMemoryRouter(
      [
        {
          path: "*",
          ErrorBoundary,
          loader: () => {
            throw new Response(null, { status: 404, statusText: "Not Found" });
          },
          element: <p>never</p>,
        },
      ],
      { initialEntries: ["/app/tests/gone"] },
    );
    render(<RouterProvider router={router} />);
    expect(
      await screen.findByRole("heading", { name: "Trang này không tồn tại" }),
    ).toBeInTheDocument();
  });
});

describe("the 403", () => {
  it("names the account it is refusing, by name", () => {
    useAuthStore.setState({ user: studentUser });
    renderAt(<ForbiddenPage />, "/admin/tests");
    expect(screen.getByText(studentUser.fullName)).toBeInTheDocument();
    expect(screen.getByText("Đang đăng nhập")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Về trang chủ của tôi" })).toHaveAttribute(
      "href",
      "/app",
    );
  });

  it("offers to switch accounts rather than to sign out", () => {
    useAuthStore.setState({ user: studentUser });
    renderAt(<ForbiddenPage />, "/admin/tests");
    expect(screen.getByText("Đăng nhập bằng tài khoản khác")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/Đăng xuất/);
  });

  it("offers a signed-out visitor sign-in, and invents no account", () => {
    renderAt(<ForbiddenPage />, "/admin/tests");
    expect(
      screen.getByRole("heading", { name: "Bạn không có quyền mở trang này" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Đang đăng nhập")).toBeNull();
    expect(screen.getByRole("link", { name: "Đăng nhập" })).toHaveAttribute(
      "href",
      "/login",
    );
    expect(screen.queryByText("Đăng nhập bằng tài khoản khác")).toBeNull();
  });
});

describe("the unexpected error", () => {
  it("promises that saved answers are safe", () => {
    renderFailure(new Error("boom"));
    expect(
      screen.getByText(
        "Sự cố nằm ở phía chúng tôi, không phải do bạn. Nếu bạn đang làm bài kiểm tra, các câu trả lời đã lưu vẫn an toàn.",
      ),
    ).toBeInTheDocument();
  });

  it("labels the error with the server's request id when there is one", () => {
    renderFailure(
      new ApiError({
        status: 500,
        code: "INTERNAL",
        message: "boom",
        requestId: REQUEST_ID,
      }),
    );
    expect(screen.getByText(REQUEST_ID)).toBeInTheDocument();
  });

  it("makes up a short id to read aloud when there is none", () => {
    renderFailure(new Error("boom"));
    const code = screen.getByText(/^[0-9a-f]{4}-[0-9a-f]{4}$/);
    expect(code).toHaveClass("select-all");
  });

  it("copies the id and says so for two seconds", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const writeText = vi.spyOn(navigator.clipboard, "writeText");
    renderFailure(
      new ApiError({
        status: 500,
        code: "INTERNAL",
        message: "boom",
        requestId: REQUEST_ID,
      }),
    );
    await user.click(screen.getByRole("button", { name: "Sao chép" }));

    expect(writeText).toHaveBeenCalledWith(REQUEST_ID);
    expect(screen.getByRole("button", { name: "Đã sao chép" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Đã sao chép");
    await act(() => vi.advanceTimersByTimeAsync(2000));
    expect(screen.getByRole("button", { name: "Sao chép" })).toBeInTheDocument();
  });

  it("says how to copy by hand when the clipboard refuses", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(new Error("denied"));
    renderFailure(new Error("boom"));
    await user.click(screen.getByRole("button", { name: "Sao chép" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Không sao chép được. Hãy chọn mã lỗi rồi sao chép.",
    );
  });

  it("shows the maintenance page for a maintenance error", () => {
    renderFailure(
      new ApiError({
        status: 503,
        code: "MAINTENANCE",
        message: "boom",
        details: { startsAt: "2026-10-01T14:30:00Z", endsAt: "2026-10-01T15:30:00Z" },
      }),
    );
    expect(
      screen.getByRole("heading", { name: "Quizzivy đang được cập nhật" }),
    ).toBeInTheDocument();
  });
});

describe("the maintenance page", () => {
  const window = { startsAt: "2026-10-01T14:30:00Z", endsAt: "2026-10-01T15:30:00Z" };

  it("gives the times in Vietnam, without a date on the same day", () => {
    renderAt(
      <MaintenancePage
        window={window}
        onCheck={() => undefined}
        now={new Date("2026-10-01T14:40:00Z")}
      />,
    );
    expect(
      screen.getByText(
        "Chúng tôi sẽ trở lại lúc 22:30. Bài kiểm tra đóng trong lúc cập nhật sẽ được gia hạn.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Bắt đầu lúc 21:30")).toBeInTheDocument();
    expect(screen.getByText("khoảng 60 phút")).toBeInTheDocument();
  });

  it("adds the date when the window ends on another day", () => {
    renderAt(
      <MaintenancePage
        window={{ startsAt: "2026-10-01T16:30:00Z", endsAt: "2026-10-01T17:30:00Z" }}
        onCheck={() => undefined}
        now={new Date("2026-10-01T16:40:00Z")}
      />,
    );
    expect(screen.getByText(/trở lại lúc 00:30, 02\/10\./)).toBeInTheDocument();
    expect(screen.getByText("Bắt đầu lúc 23:30")).toBeInTheDocument();
  });

  it("looks again when asked", async () => {
    const onCheck = vi.fn();
    renderAt(<MaintenancePage window={window} onCheck={onCheck} />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Kiểm tra lại" }));
    expect(onCheck).toHaveBeenCalledOnce();
  });
});

describe("the brand kit on screen", () => {
  // B-05: never the colour variant on a dark surface.
  it("uses the on-dark lockup wherever the surface is dark", () => {
    render(<BrandLockup height={44} onDark />);
    expect(screen.getByAltText("Quizzivy")).toHaveAttribute(
      "src",
      "/brand/quizzivy-logo-horizontal-on-dark.svg",
    );
  });

  it("sizes from the kit's own viewBox", () => {
    render(<BrandLockup height={28} />);
    const img = screen.getByAltText("Quizzivy");
    // 885.5 / 205 = 4.3195 -> 121 at h=28, which clears the 120px floor.
    expect(img).toHaveAttribute("width", "121");
    expect(img).toHaveAttribute("height", "28");
  });

  it("warns about an undersized mark but still renders it", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    expect(() => render(<BrandMark height={12} />)).not.toThrow();
    expect(screen.getByText("Quizzivy")).toBeInTheDocument();
    if (import.meta.env.DEV) expect(warn).toHaveBeenCalled();
  });

  it("leaves the mark decorative when the app writes the name beside it", () => {
    const { container } = render(<BrandMark height={24} />);
    expect(container.querySelector("img")).toHaveAttribute("alt", "");
    expect(screen.getByText("Quizzivy")).toBeInTheDocument();
  });
});
