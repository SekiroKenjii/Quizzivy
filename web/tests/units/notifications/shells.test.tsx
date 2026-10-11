import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import AdminLayout from "@/layouts/AdminLayout";
import { useAuthStore } from "@/stores/auth";
import {
  home,
  renderRoutes,
  renderShell,
  serveDashboard,
  serveSummary,
  summaryBody,
} from "../shell/support";
import "@/lib/i18n";

const flags = vi.hoisted(() => ({
  notifications: true,
  messages: false,
  schedule: false,
  grades: false,
  learn: false,
}));
vi.mock("@/app/modules", () => ({ modules: flags }));

function bell() {
  return screen.queryByRole("button", { name: /^Thông báo/ });
}

afterEach(() => {
  flags.notifications = true;
  useAuthStore.getState().clearSession();
});

describe("the teacher shell's bell", () => {
  it("sits in the top bar with a dot from the summary's unread count", async () => {
    serveDashboard();
    serveSummary({ ...summaryBody, unreadNotifications: 2 });
    renderShell("/teacher", [home()]);
    const button = await screen.findByRole("button", { name: "Thông báo, 2 chưa đọc" });
    expect(button.closest("header")).not.toBeNull();
    expect(button.querySelector("[data-slot='bell-dot']")).not.toBeNull();
  });

  it("has no dot when nothing is unread", async () => {
    serveDashboard();
    const seen = serveSummary();
    renderShell("/teacher", [home()]);
    await waitFor(() => expect(seen.asked).toBeGreaterThan(0));
    const button = await screen.findByRole("button", { name: "Thông báo" });
    expect(button.querySelector("[data-slot='bell-dot']")).toBeNull();
  });

  it("is absent while the notifications module is off", async () => {
    flags.notifications = false;
    serveDashboard();
    serveSummary({ ...summaryBody, unreadNotifications: 2 });
    renderShell("/teacher", [home()]);
    expect(await screen.findByText("trang tổng quan")).toBeInTheDocument();
    expect(bell()).toBeNull();
  });
});

describe("the admin console's bell", () => {
  const admin = () => [
    {
      path: "/teacher",
      element: <AdminLayout />,
      children: [{ index: true, element: <p>bảng điều khiển cũ</p> }],
    },
  ];

  it("keeps a bell until R5 rebuilds the console", async () => {
    serveDashboard();
    serveSummary({ ...summaryBody, unreadNotifications: 1 });
    renderRoutes("/teacher", admin());
    expect(
      await screen.findByRole("button", { name: "Thông báo, 1 chưa đọc" }),
    ).toBeInTheDocument();
  });

  it("has none while the notifications module is off", async () => {
    flags.notifications = false;
    serveDashboard();
    serveSummary();
    renderRoutes("/teacher", admin());
    expect(await screen.findByText("bảng điều khiển cũ")).toBeInTheDocument();
    expect(bell()).toBeNull();
  });
});
