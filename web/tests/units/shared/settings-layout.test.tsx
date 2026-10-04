import { afterEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Bell, Lock, User } from "lucide-react";
import { createMemoryRouter, RouterProvider, useParams } from "react-router";
import {
  SettingsLayout,
  type SettingsSection,
} from "@/components/shared/SettingsLayout";
import "@/lib/i18n";

const SECTIONS: readonly SettingsSection[] = [
  {
    id: "profile",
    label: "Hồ sơ",
    icon: User,
    to: "/teacher/settings",
    content: (
      <label>
        Họ và tên
        <input defaultValue="Hoàng Thương" />
      </label>
    ),
  },
  {
    id: "security",
    label: "Đăng nhập và bảo mật",
    icon: Lock,
    to: "/teacher/settings/security",
    content: <p>Mật khẩu</p>,
  },
  {
    id: "notifications",
    label: "Thông báo",
    icon: Bell,
    to: "/teacher/settings/notifications",
    content: <p>Bài nộp mới</p>,
  },
];

function Page() {
  const { section } = useParams();
  return (
    <SettingsLayout label="Cài đặt" active={section ?? "profile"} sections={SECTIONS} />
  );
}

function at(path: string) {
  const router = createMemoryRouter(
    [
      { path: "/teacher/settings", element: <Page /> },
      { path: "/teacher/settings/:section", element: <Page /> },
    ],
    { initialEntries: [path] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

const nav = () => screen.getByRole("navigation", { name: "Cài đặt" });
const link = (name: string) => within(nav()).getByRole("link", { name });
const panel = (id: string) =>
  document.querySelector<HTMLElement>(`[data-section="${id}"]`)!;

function place(element: Element, left: number, right: number, moved = () => 0) {
  vi.spyOn(element, "getBoundingClientRect").mockImplementation(() => ({
    left: left - moved(),
    right: right - moved(),
    width: right - left,
    top: 0,
    bottom: 36,
    height: 36,
    x: left - moved(),
    y: 0,
    toJSON: () => ({}),
  }));
}

function Phone({ active }: Readonly<{ active: string }>) {
  return (
    <div
      ref={(wrapper) => {
        const frame = wrapper?.querySelector("nav");
        if (!frame) return;
        const moved = () => frame.scrollLeft;
        place(frame, 0, 300);
        const [profile, security, notifications] = frame.querySelectorAll("a");
        place(profile!, 0, 90, moved);
        place(security!, 92, 280, moved);
        place(notifications!, 282, 400, moved);
      }}
    >
      <SettingsLayout label="Cài đặt" active={active} sections={SECTIONS} />
    </div>
  );
}

function phoneAt(active: string) {
  const router = createMemoryRouter([
    { path: "*", element: <Phone active={active} /> },
  ]);
  return render(<RouterProvider router={router} />);
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("the settings navigation", () => {
  it("is a navigation with one link per section, each to its own address", () => {
    at("/teacher/settings");
    expect(within(nav()).getAllByRole("link")).toHaveLength(3);
    expect(link("Hồ sơ")).toHaveAttribute("href", "/teacher/settings");
    expect(link("Đăng nhập và bảo mật")).toHaveAttribute(
      "href",
      "/teacher/settings/security",
    );
    expect(link("Thông báo")).toHaveAttribute(
      "href",
      "/teacher/settings/notifications",
    );
    expect(link("Hồ sơ").querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("marks the current section's link and no other", () => {
    at("/teacher/settings/security");
    expect(link("Đăng nhập và bảo mật")).toHaveAttribute("aria-current", "page");
    expect(link("Hồ sơ")).not.toHaveAttribute("aria-current");
    expect(link("Thông báo")).not.toHaveAttribute("aria-current");
  });

  it("fills the current link and leaves the others muted", () => {
    at("/teacher/settings/security");
    expect(link("Đăng nhập và bảo mật")).toHaveClass(
      "bg-hover",
      "text-fg",
      "font-medium",
    );
    expect(link("Hồ sơ")).toHaveClass(
      "text-muted-fg",
      "hover:bg-hover",
      "hover:text-fg",
    );
    expect(link("Hồ sơ")).not.toHaveClass("bg-hover");
    expect(link("Hồ sơ")).not.toHaveClass("font-medium");
  });

  it("draws a 36px link with a 16px icon, a 10px gap and a ring inside its box", () => {
    at("/teacher/settings");
    expect(link("Hồ sơ")).toHaveClass(
      "h-9",
      "gap-2.5",
      "rounded-md",
      "px-3",
      "text-ui",
      "whitespace-nowrap",
      "flex-none",
      "-outline-offset-2!",
    );
    expect(link("Hồ sơ").querySelector("svg")).toHaveClass("size-4");
  });

  it("is a scrolling row below 768 and a column of at least 200px from there", () => {
    at("/teacher/settings");
    expect(nav()).toHaveClass(
      "flex",
      "flex-[0_1_100%]",
      "max-w-full",
      "gap-0.5",
      "overflow-x-auto",
      "min-[768px]:flex-col",
      "min-[768px]:flex-none",
      "min-[768px]:min-w-50",
    );
    expect(nav().className).not.toMatch(/(^|\s)(md|lg):/);
    expect(nav().parentElement).toHaveClass(
      "flex",
      "flex-wrap",
      "items-start",
      "gap-6",
    );
  });

  it("goes to a section through its link", async () => {
    const user = userEvent.setup();
    const router = at("/teacher/settings");
    await user.click(link("Thông báo"));
    expect(router.state.location.pathname).toBe("/teacher/settings/notifications");
    expect(link("Thông báo")).toHaveAttribute("aria-current", "page");
  });
});

describe("the settings sections", () => {
  it("renders every section and shows only the current one", () => {
    at("/teacher/settings/security");
    expect(screen.getByText("Mật khẩu")).toBeVisible();
    expect(screen.getByText("Họ và tên")).toBeInTheDocument();
    expect(screen.getByText("Họ và tên")).not.toBeVisible();
    expect(screen.getByText("Bài nộp mới")).toBeInTheDocument();
    expect(screen.getByText("Bài nộp mới")).not.toBeVisible();
    expect(panel("security")).not.toHaveAttribute("hidden");
    expect(panel("profile")).toHaveAttribute("hidden");
    expect(panel("notifications")).toHaveAttribute("hidden");
  });

  it("stacks a section's cards 14px apart beside the navigation", () => {
    at("/teacher/settings");
    expect(panel("profile")).toHaveClass("flex", "flex-col", "gap-3.5");
    expect(panel("profile").parentElement).toHaveClass("flex-[1_1_480px]", "min-w-0");
  });

  it("keeps what was typed in one section while another is shown", async () => {
    const user = userEvent.setup();
    const router = at("/teacher/settings");
    const name = screen.getByLabelText("Họ và tên");
    await user.clear(name);
    await user.type(name, "Võ Hoàng Thương");

    await user.click(link("Đăng nhập và bảo mật"));
    expect(router.state.location.pathname).toBe("/teacher/settings/security");
    expect(screen.getByText("Mật khẩu")).toBeVisible();
    expect(screen.getByLabelText("Họ và tên")).not.toBeVisible();

    await user.click(link("Hồ sơ"));
    expect(router.state.location.pathname).toBe("/teacher/settings");
    expect(screen.getByLabelText("Họ và tên")).toBeVisible();
    expect(screen.getByLabelText("Họ và tên")).toHaveValue("Võ Hoàng Thương");
    expect(screen.getByLabelText("Họ và tên")).toBe(name);
  });

  it("shows the first section for an address that names none", () => {
    at("/teacher/settings/khong-co");
    expect(screen.getByText("Họ và tên")).toBeVisible();
    expect(screen.getByText("Mật khẩu")).not.toBeVisible();
    expect(link("Hồ sơ")).toHaveAttribute("aria-current", "page");
    expect(link("Đăng nhập và bảo mật")).not.toHaveAttribute("aria-current");
  });
});

describe("the settings navigation as a row", () => {
  it("brings the current link into view when the edge cuts it", () => {
    phoneAt("notifications");
    expect(nav().scrollLeft).toBe(100);
  });

  it("leaves the row where it is when the current link is in view", () => {
    phoneAt("security");
    expect(nav().scrollLeft).toBe(0);
  });

  it("brings a link that takes focus into view without choosing it", () => {
    phoneAt("profile");
    expect(nav().scrollLeft).toBe(0);
    act(() => link("Thông báo").focus());
    expect(nav().scrollLeft).toBe(100);
    expect(link("Thông báo")).not.toHaveAttribute("aria-current");
  });
});
