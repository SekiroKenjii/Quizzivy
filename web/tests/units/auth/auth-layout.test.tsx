import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import "@/lib/i18n";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

async function renderLayout(props: { panel?: boolean } = {}) {
  const { AuthLayout } = await import("@/features/auth/AuthLayout");
  const { queryWrapper } = await import("@tests/support/queryWrapper");
  return render(
    <AuthLayout {...props}>
      <h1>Đăng nhập</h1>
    </AuthLayout>,
    { wrapper: queryWrapper() },
  );
}

describe("the sign-in frame", () => {
  it("shows the brand panel from the 900px breakpoint, on the page background", async () => {
    const { container } = await renderLayout();
    const panel = container.querySelector("aside");
    expect(panel).toHaveClass("hidden", "auth:flex");
    expect(container.firstElementChild).toHaveClass("bg-bg", "text-fg", "min-h-svh");
    expect(
      screen.getByText("Bài kiểm tra, bài học và luyện tập cho lớp tiếng Anh của bạn."),
    ).toBeInTheDocument();
  });

  it("stays one column when the page asks for no panel", async () => {
    const { container } = await renderLayout({ panel: false });
    expect(container.querySelector("aside")).toBeNull();
  });

  it("hides the organisation line until it is configured", async () => {
    const plain = await renderLayout();
    expect(plain.container.textContent).not.toContain("Quizzivy English Centre");
    plain.unmount();
    vi.resetModules();
    vi.stubEnv("VITE_ORG_NAME", "Quizzivy English Centre");
    await renderLayout();
    expect(screen.getByText("Quizzivy English Centre")).toBeInTheDocument();
  });
});
