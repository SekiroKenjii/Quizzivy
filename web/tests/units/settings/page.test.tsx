import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useAuthStore } from "@/stores/auth";
import { adminUser, teacherUser } from "@tests/support/fixtures";
import { contentWidth } from "@tests/support/contentWidth";
import { THIS_DEVICE, listsSessions, renderSettings, signIn } from "./support";
import "@/lib/i18n";

beforeEach(() => {
  contentWidth(1080);
  listsSessions([THIS_DEVICE]);
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function nav() {
  return within(screen.getByRole("navigation", { name: "Mục cài đặt" }));
}

describe("the teacher's Settings", () => {
  it("heads the page as the deck does and lists the shipped sections", () => {
    signIn(teacherUser);
    renderSettings();

    expect(screen.getByRole("heading", { level: 1, name: "Cài đặt" })).toBeVisible();
    expect(
      screen.getByText("Hồ sơ, đăng nhập và cách Quizzivy làm việc cùng bạn."),
    ).toBeVisible();
    expect(
      nav()
        .getAllByRole("link")
        .map((link) => link.textContent),
    ).toEqual(["Hồ sơ", "Đăng nhập & bảo mật", "Giao diện"]);
    expect(nav().getByRole("link", { name: "Hồ sơ" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("adds API reference only for who may open it", () => {
    signIn(adminUser);
    renderSettings("/teacher/settings/api");

    expect(nav().getByRole("link", { name: "Tài liệu API" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("button", { name: "Mở tài liệu API" })).toBeVisible();
  });

  it.each([
    ["the pre-R4 preferences", "/teacher/settings/preferences"],
    ["API reference without the permission", "/teacher/settings/api"],
    ["an unknown section", "/teacher/settings/nope"],
  ])("replaces %s with Profile", async (_, path) => {
    signIn(teacherUser);
    const { router } = renderSettings(path);

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/settings"),
    );
    expect(router.state.historyAction).toBe("REPLACE");
    expect(screen.getByRole("textbox", { name: "Họ và tên" })).toBeVisible();
  });

  it("opens each section at its own address", () => {
    signIn(teacherUser);
    renderSettings("/teacher/settings/appearance");

    expect(screen.getByRole("heading", { name: "Giao diện" })).toBeVisible();
    expect(
      screen.getByRole("textbox", { name: "Họ và tên", hidden: true }),
    ).not.toBeVisible();
  });

  it("keeps an unsaved profile while another section is shown", async () => {
    signIn(teacherUser);
    renderSettings();
    const user = userEvent.setup();

    const name = screen.getByRole("textbox", { name: "Họ và tên" });
    await user.clear(name);
    await user.type(name, "Tên chưa lưu");
    await user.click(nav().getByRole("link", { name: "Đăng nhập & bảo mật" }));
    expect(
      await screen.findByRole("heading", { name: "Thiết bị đã đăng nhập" }),
    ).toBeVisible();
    await user.click(nav().getByRole("link", { name: "Hồ sơ" }));

    expect(screen.getByRole("textbox", { name: "Họ và tên" })).toHaveValue(
      "Tên chưa lưu",
    );
  });
});
