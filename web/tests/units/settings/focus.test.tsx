import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import {
  GoogleSection,
  PasswordSection,
} from "@/features/auth/components/SettingsSections";
import { AppearanceSection } from "@/features/settings/sections/Appearance";
import { DevicesSection } from "@/features/settings/sections/Devices";
import { ProfileSection } from "@/features/settings/sections/Profile";
import { __resetRefreshStateForTests } from "@/lib/api/client";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { BASE, PHONE, THIS_DEVICE, UNKNOWN, signIn } from "./support";
import "@/lib/i18n";

const SAVED = {
  ...teacherUser,
  fullName: "Hoàng Thương",
  displayName: "Ms Thương",
  phone: "0912 345 678",
  locale: "vi" as const,
  timeZone: "Asia/Ho_Chi_Minh",
  preferences: { theme: "light" as const },
};

beforeEach(() => {
  contentWidth(1080);
  __resetRefreshStateForTests();
  signIn(SAVED);
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

function withClient(children: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      {children}
      <Toaster />
    </QueryClientProvider>,
  );
}

const field = (name: string) => screen.getByRole("textbox", { name });

describe("focus in Appearance while a choice saves", () => {
  function savesPreferences(reply: Promise<void>) {
    server.use(
      http.patch(`${BASE}/me/preferences`, async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        await reply;
        return HttpResponse.json({ theme: "light", ...body });
      }),
    );
  }

  it("stays on the theme swatch that was pressed", async () => {
    const reply = gate();
    savesPreferences(reply.opened);
    withClient(<AppearanceSection />);
    const user = userEvent.setup();
    const dark = screen.getByRole("button", { name: "Tối" });

    await user.click(dark);

    expect(dark).toHaveFocus();
    expect(dark).toBeEnabled();
    expect(dark).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByRole("button", { name: "Sáng" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    reply.open();
    await waitFor(() =>
      expect(useAuthStore.getState().user?.preferences?.theme).toBe("dark"),
    );
    expect(dark).toHaveFocus();
    expect(dark).not.toHaveAttribute("aria-disabled");
  });

  it("ignores a second swatch while the first saves", async () => {
    const reply = gate();
    const bodies: unknown[] = [];
    server.use(
      http.patch(`${BASE}/me/preferences`, async ({ request }) => {
        bodies.push(await request.json());
        await reply.opened;
        return HttpResponse.json({ theme: "dark" });
      }),
    );
    withClient(<AppearanceSection />);
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Tối" }));
    await user.click(screen.getByRole("button", { name: "Theo thiết bị" }));
    reply.open();

    await waitFor(() => expect(bodies).toEqual([{ theme: "dark" }]));
    expect(screen.getByRole("button", { name: "Tối" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("stays on Compact tables, by click and by Space", async () => {
    const reply = gate();
    savesPreferences(reply.opened);
    withClient(<AppearanceSection />);
    const user = userEvent.setup();
    const compact = screen.getByRole("switch", { name: "Bảng thu gọn" });

    await user.click(compact);

    expect(compact).toHaveFocus();
    expect(compact).toBeEnabled();
    expect(compact).toHaveAttribute("aria-disabled", "true");
    await user.keyboard(" ");
    expect(compact).toBeChecked();
    reply.open();
    await waitFor(() => expect(compact).not.toHaveAttribute("aria-disabled"));
    expect(compact).toHaveFocus();
    expect(compact).toBeChecked();
  });
});

describe("focus in Profile when the unsaved-changes bar goes", () => {
  function patches(reply?: Promise<void>) {
    server.use(
      http.patch(`${BASE}/auth/me`, async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        if (reply) await reply;
        return contractJson("/auth/me", "patch", 200, { ...SAVED, ...body });
      }),
    );
  }

  it("stays on Save changes while it saves, then moves to the field last edited", async () => {
    const reply = gate();
    patches(reply.opened);
    withClient(<ProfileSection />);
    const user = userEvent.setup();

    await user.type(field("Họ và tên"), " Hoàng");
    await user.clear(field("Số điện thoại"));
    await user.type(field("Số điện thoại"), "0987 654 321");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    const saving = await screen.findByRole("button", { name: "Đang lưu…" });
    expect(saving).toHaveFocus();
    expect(saving).toBeEnabled();
    expect(saving).toHaveAttribute("aria-disabled", "true");
    reply.open();

    expect(await screen.findByText("Đã lưu cài đặt")).toBeInTheDocument();
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(field("Số điện thoại")).toHaveFocus();
  });

  it("moves to the field last edited after Discard", async () => {
    withClient(<ProfileSection />);
    const user = userEvent.setup();

    await user.type(field("Tên học viên thấy"), "!");
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));

    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(field("Tên học viên thấy")).toHaveValue("Ms Thương");
    expect(field("Tên học viên thấy")).toHaveFocus();
  });

  it("moves to the time zone after Discard when the zone was the last edit", async () => {
    withClient(<ProfileSection />);
    const user = userEvent.setup();

    await user.type(field("Họ và tên"), "!");
    await user.click(screen.getByRole("combobox", { name: "Múi giờ" }));
    await user.click(screen.getByRole("option", { name: /Tokyo$/ }));
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));

    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(screen.getByRole("combobox", { name: "Múi giờ" })).toHaveFocus();
  });

  it("keeps Save changes in place when the save fails", async () => {
    server.use(
      http.patch(`${BASE}/auth/me`, () => HttpResponse.json(null, { status: 500 })),
    );
    withClient(<ProfileSection />);
    const user = userEvent.setup();

    await user.type(field("Họ và tên"), "!");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Lưu thay đổi" })).toHaveFocus();
  });
});

describe("focus in Sign-in & security", () => {
  it("stays on Change password while it is changing", async () => {
    const reply = gate();
    server.use(
      http.post(`${BASE}/auth/change-password`, async () => {
        await reply.opened;
        return new HttpResponse(null, { status: 204 });
      }),
      http.get(`${BASE}/auth/me`, () =>
        contractJson("/auth/me", "get", 200, { ...SAVED }),
      ),
    );
    withClient(<PasswordSection />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText("Mật khẩu hiện tại"), "old-password-1");
    await user.type(screen.getByLabelText("Mật khẩu mới"), "new-password-2");
    const submit = screen.getByRole("button", { name: "Đổi mật khẩu" });
    await user.click(submit);

    const changing = await screen.findByRole("button", { name: "Đang lưu…" });
    expect(changing).toHaveFocus();
    expect(changing).toBeEnabled();
    expect(changing).toHaveAttribute("aria-disabled", "true");
    reply.open();
    expect(await screen.findByRole("button", { name: "Đổi mật khẩu" })).toHaveFocus();
  });

  it("moves to the Not linked line after Google is unlinked", async () => {
    signIn({ ...SAVED, hasPassword: true, linkedProviders: ["google"] });
    server.use(
      http.delete(
        `${BASE}/auth/google/link`,
        () => new HttpResponse(null, { status: 204 }),
      ),
      http.get(`${BASE}/auth/me`, () =>
        contractJson("/auth/me", "get", 200, {
          ...SAVED,
          hasPassword: true,
          linkedProviders: [],
        }),
      ),
    );
    withClient(<GoogleSection />);
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Bỏ liên kết Google" }));

    const line = await screen.findByText("Chưa liên kết với Google.");
    expect(line).toHaveFocus();
  });
});

describe("focus in Signed-in devices", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-10-10T10:00:00Z") });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function signsOut(initial: (typeof PHONE)[]) {
    let items = initial;
    const reply = gate();
    server.use(
      http.get(`${BASE}/auth/sessions`, () =>
        contractJson("/auth/sessions", "get", 200, { items }),
      ),
      http.delete(`${BASE}/auth/sessions/:familyId`, async ({ params }) => {
        await reply.opened;
        items = items.filter((session) => session.familyId !== params.familyId);
        return new HttpResponse(null, { status: 204 });
      }),
    );
    return reply;
  }

  async function signOutButton(name: string) {
    const list = await screen.findByRole("list", { name: "Thiết bị đã đăng nhập" });
    return within(list).getByRole("button", { name: `Đăng xuất ${name}` });
  }

  it("keeps Sign out focused while it is pending, then moves to the next row's", async () => {
    const reply = signsOut([THIS_DEVICE, PHONE, UNKNOWN]);
    withClient(<DevicesSection />);
    const user = userEvent.setup();

    const phone = await signOutButton("iPhone · Safari");
    await user.click(phone);

    expect(phone).toHaveFocus();
    expect(phone).toBeEnabled();
    expect(phone).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByRole("button", { name: "Đăng xuất các thiết bị khác" }),
    ).toHaveAttribute("aria-disabled", "true");
    reply.open();

    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Đăng xuất iPhone · Safari" }),
      ).toBeNull(),
    );
    await waitFor(async () =>
      expect(await signOutButton("Thiết bị không xác định")).toHaveFocus(),
    );
  });

  it("moves to the previous row's Sign out when the last row goes", async () => {
    const reply = signsOut([THIS_DEVICE, PHONE, UNKNOWN]);
    reply.open();
    withClient(<DevicesSection />);
    const user = userEvent.setup();

    await user.click(await signOutButton("Thiết bị không xác định"));

    await waitFor(async () =>
      expect(await signOutButton("iPhone · Safari")).toHaveFocus(),
    );
  });

  it("moves to Sign out other devices when no other row is left", async () => {
    const reply = signsOut([THIS_DEVICE, PHONE]);
    reply.open();
    withClient(<DevicesSection />);
    const user = userEvent.setup();

    await user.click(await signOutButton("iPhone · Safari"));

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Đăng xuất các thiết bị khác" }),
      ).toHaveFocus(),
    );
  });
});
