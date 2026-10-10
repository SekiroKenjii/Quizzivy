import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import { AppearanceSection } from "@/features/settings/sections/Appearance";
import { ProfileSection } from "@/features/settings/sections/Profile";
import { readCompactTables } from "@/lib/compactTables";
import { useAuthStore } from "@/stores/auth";
import { contentWidth } from "@tests/support/contentWidth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { BASE, signIn } from "./support";
import "@/lib/i18n";

beforeEach(() => {
  contentWidth(1080);
  signIn({ ...teacherUser, preferences: { theme: "light" } });
});

afterEach(() => {
  useAuthStore.getState().clearSession();
  localStorage.clear();
});

function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { open, opened };
}

function savesPreferences(reply?: Promise<void>) {
  const bodies: unknown[] = [];
  server.use(
    http.patch(`${BASE}/me/preferences`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      bodies.push(body);
      if (reply) await reply;
      return HttpResponse.json({ theme: "light", ...body });
    }),
  );
  return bodies;
}

function renderAppearance() {
  render(
    <>
      <ProfileSection />
      <AppearanceSection />
      <Toaster />
    </>,
  );
}

describe("the Appearance card", () => {
  it("offers Light, Dark and Match device, the last as a split swatch", () => {
    renderAppearance();

    const themes = within(screen.getByRole("group", { name: "Chủ đề" }));
    const buttons = themes.getAllByRole("button");
    expect(buttons.map((button) => button.textContent)).toEqual([
      "Sáng",
      "Tối",
      "Theo thiết bị",
    ]);
    expect(buttons[0]).toHaveAttribute("aria-pressed", "true");
    const split = buttons[2]!.querySelector('[data-swatch="system"]')!;
    expect(split.querySelector(".bg-swatch-light")).not.toBeNull();
    expect(split.querySelector(".bg-swatch-dark")).not.toBeNull();
  });

  it("applies a theme at once and saves it without raising the DirtyBar", async () => {
    const bodies = savesPreferences();
    renderAppearance();
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Tối" }));

    expect(document.documentElement).toHaveClass("dark");
    await waitFor(() => expect(bodies).toEqual([{ theme: "dark" }]));
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
  });

  it("applies Compact tables before the server answers, and Discard leaves it alone", async () => {
    const reply = gate();
    const bodies = savesPreferences(reply.opened);
    renderAppearance();
    const user = userEvent.setup();

    await user.type(screen.getByRole("textbox", { name: "Họ và tên" }), " mới");
    await user.click(screen.getByRole("switch", { name: "Bảng thu gọn" }));

    expect(readCompactTables()).toBe(true);
    expect(screen.getByRole("switch", { name: "Bảng thu gọn" })).toBeChecked();
    reply.open();
    await waitFor(() =>
      expect(useAuthStore.getState().user?.preferences?.compactTables).toBe(true),
    );
    expect(bodies).toEqual([{ compactTables: true }]);

    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));
    expect(readCompactTables()).toBe(true);
    expect(screen.getByRole("switch", { name: "Bảng thu gọn" })).toBeChecked();
  });
});
