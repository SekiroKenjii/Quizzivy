import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { NotFound } from "@/app/ErrorBoundary";
import ForbiddenPage from "@/app/pages/ForbiddenPage";
import { BrandLockup, BrandMark } from "@/components/shared/Brand";
import { useAuthStore } from "@/stores/auth";
import { studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

/**
 * E-01..E-04. Each assertion is a sentence from the deck that the screen would
 * otherwise silently stop honouring.
 */

function renderAt(element: React.ReactElement, path = "/app/assignments/8f2c-unit-5") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "*", element }], {
    initialEntries: [path],
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

afterEach(() => {
  useAuthStore.setState({ user: null });
  vi.restoreAllMocks();
});

describe("the three failure screens", () => {
  it("puts the address that failed on the 404 itself", () => {
    renderAt(<NotFound />, "/app/assignments/8f2c-unit-5");
    expect(screen.getByText("/app/assignments/8f2c-unit-5")).toBeInTheDocument();
  });

  // E-01: home always works, back often returns to the same dead link.
  it("ranks home above back", () => {
    renderAt(<NotFound />);
    const home = screen.getByText("Về trang chủ");
    const back = screen.getByText("Quay lại");
    expect(
      home.compareDocumentPosition(back) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  // §12 and E-01: the number means nothing to a fifteen-year-old.
  it("never prints the status number", () => {
    renderAt(<NotFound />);
    expect(document.body.textContent).not.toMatch(/\b404\b/);
  });

  it("names the account the 403 is refusing", () => {
    useAuthStore.setState({ user: studentUser });
    renderAt(<ForbiddenPage />, "/admin");
    expect(screen.getByText(studentUser.email)).toBeInTheDocument();
  });

  // E-03: sign-out states what it takes away; this states what it gets them.
  it("offers to switch accounts rather than to sign out", () => {
    useAuthStore.setState({ user: studentUser });
    renderAt(<ForbiddenPage />, "/admin");
    expect(screen.getByText("Đăng nhập bằng tài khoản khác")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/Đăng xuất/);
  });

  it("renders the 403 for a signed-out visitor without inventing an account", () => {
    renderAt(<ForbiddenPage />, "/admin");
    expect(screen.getByText("Bạn không có quyền truy cập")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/Đang đăng nhập bằng/);
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
