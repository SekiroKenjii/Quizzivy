import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import { DevicesSection } from "@/features/settings/sections/Devices";
import { __resetRefreshStateForTests } from "@/lib/api/client";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import {
  BASE,
  PHONE,
  REQUEST_ID,
  THIS_DEVICE,
  UNKNOWN,
  listsSessions,
  signIn,
} from "./support";
import "@/lib/i18n";

const NOW = new Date("2026-10-10T10:00:00Z");

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: NOW });
  __resetRefreshStateForTests();
  signIn(teacherUser);
});

afterEach(() => {
  vi.useRealTimers();
  useAuthStore.getState().clearSession();
});

function renderDevices() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <DevicesSection />
      <Toaster />
    </QueryClientProvider>,
  );
}

function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { open, opened };
}

function unauthorized(path: string, method: "get" | "post" | "delete") {
  return contractJson(path, method, 401, {
    error: { code: "UNAUTHORIZED", message: "Hết phiên.", requestId: REQUEST_ID },
  });
}

async function rows() {
  return within(
    await screen.findByRole("list", { name: "Thiết bị đã đăng nhập" }),
  ).getAllByRole("listitem");
}

describe("the signed-in devices", () => {
  it("lists the sessions with the server's labels, the calling one first", async () => {
    listsSessions([THIS_DEVICE, PHONE, UNKNOWN]);
    renderDevices();

    const [mine, phone, unknown] = await rows();
    expect(mine).toHaveTextContent("Mac · ChromeThiết bị này");
    expect(mine).toHaveTextContent("Ho Chi Minh City, VN · bây giờ");
    expect(within(mine!).queryByRole("button")).toBeNull();
    expect(phone).toHaveTextContent("iPhone · Safari");
    expect(phone).toHaveTextContent("Da Nang, VN · 2 giờ trước");
    expect(unknown).toHaveTextContent("Thiết bị không xác định");
    expect(unknown).toHaveTextContent(/^Thiết bị không xác định5 ngày trước/);
    expect(
      within(unknown!).getByRole("button", {
        name: "Đăng xuất Thiết bị không xác định",
      }),
    ).toBeEnabled();
  });

  it("shows skeleton rows while loading, and a retry when the list fails", async () => {
    let answer: "fail" | "ok" = "fail";
    server.use(
      http.get(`${BASE}/auth/sessions`, () =>
        answer === "fail"
          ? HttpResponse.json(
              { error: { code: "INTERNAL", message: "Lỗi.", requestId: REQUEST_ID } },
              { status: 500 },
            )
          : contractJson("/auth/sessions", "get", 200, { items: [THIS_DEVICE] }),
      ),
    );
    renderDevices();

    expect(screen.getByRole("status", { name: "Đang tải…" })).toBeInTheDocument();
    expect(await screen.findByText("Không tải được danh sách thiết bị.")).toBeVisible();
    answer = "ok";
    await userEvent.setup().click(screen.getByRole("button", { name: "Thử lại" }));

    expect(await rows()).toHaveLength(1);
  });

  it("turns every Sign out off while one is pending, then reads the list once through one refresh", async () => {
    const reply = gate();
    const reads: (string | null)[] = [];
    let refreshes = 0;
    let revoked = false;
    server.use(
      http.get(`${BASE}/auth/sessions`, ({ request }) => {
        const token = request.headers.get("Authorization");
        reads.push(token);
        if (revoked && token === "Bearer token-a")
          return unauthorized("/auth/sessions", "get");
        return contractJson("/auth/sessions", "get", 200, {
          items: revoked ? [THIS_DEVICE, UNKNOWN] : [THIS_DEVICE, PHONE, UNKNOWN],
        });
      }),
      http.delete(`${BASE}/auth/sessions/:familyId`, async () => {
        await reply.opened;
        revoked = true;
        return new HttpResponse(null, { status: 204 });
      }),
      http.post(`${BASE}/auth/refresh`, () => {
        refreshes += 1;
        return contractJson("/auth/refresh", "post", 200, {
          accessToken: "token-b",
          expiresIn: 900,
        });
      }),
    );
    renderDevices();
    const user = userEvent.setup();

    const [, phone, unknown] = await rows();
    await user.click(within(phone!).getByRole("button", { name: /^Đăng xuất/ }));

    expect(
      within(phone!).getByRole("button", { name: /^Đăng xuất/ }),
    ).toHaveTextContent("Đang đăng xuất…");
    expect(within(phone!).getByRole("button", { name: /^Đăng xuất/ })).toBeDisabled();
    expect(within(unknown!).getByRole("button", { name: /^Đăng xuất/ })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Đăng xuất các thiết bị khác" }),
    ).toBeDisabled();

    reply.open();
    expect(await screen.findByText("Đã đăng xuất thiết bị.")).toBeInTheDocument();
    await waitFor(async () => expect(await rows()).toHaveLength(2));
    expect(refreshes).toBe(1);
    expect(reads).toEqual(["Bearer token-a", "Bearer token-a", "Bearer token-b"]);
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Đăng xuất các thiết bị khác" }),
      ).toBeEnabled(),
    );
  });

  it("shows the server's own words when the session is the calling one", async () => {
    listsSessions([PHONE]);
    server.use(
      http.delete(`${BASE}/auth/sessions/:familyId`, () =>
        contractJson("/auth/sessions/{familyId}", "delete", 409, {
          error: {
            code: "SESSION_IS_CURRENT",
            message: "Đây là phiên bạn đang dùng. Hãy đăng xuất bằng menu tài khoản.",
            requestId: REQUEST_ID,
          },
        }),
      ),
    );
    renderDevices();
    const user = userEvent.setup();

    const [phone] = await rows();
    await user.click(within(phone!).getByRole("button", { name: /^Đăng xuất/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Đây là phiên bạn đang dùng. Hãy đăng xuất bằng menu tài khoản.",
    );
  });

  it.each([
    [0, "Không có thiết bị nào khác đang đăng nhập."],
    [1, "Đã đăng xuất 1 thiết bị khác."],
    [2, "Đã đăng xuất 2 thiết bị khác."],
  ])(
    "signs out the others after a confirmation and says so for %i",
    async (n, said) => {
      listsSessions([THIS_DEVICE, PHONE, UNKNOWN]);
      let asked = 0;
      server.use(
        http.post(`${BASE}/auth/sessions/revoke-others`, () => {
          asked += 1;
          return contractJson("/auth/sessions/revoke-others", "post", 200, {
            revoked: n,
          });
        }),
      );
      renderDevices();
      const user = userEvent.setup();
      await rows();

      await user.click(
        screen.getByRole("button", { name: "Đăng xuất các thiết bị khác" }),
      );
      const dialog = await screen.findByRole("dialog", {
        name: "Đăng xuất các thiết bị khác?",
      });
      expect(
        within(dialog).getByText("Mọi phiên đăng nhập trừ phiên này sẽ kết thúc ngay."),
      ).toBeVisible();
      expect(asked).toBe(0);
      await user.click(within(dialog).getByRole("button", { name: "Đăng xuất" }));

      expect(await screen.findByText(said)).toBeInTheDocument();
      expect(asked).toBe(1);
      await waitFor(() =>
        expect(
          screen.queryByRole("dialog", { name: "Đăng xuất các thiết bị khác?" }),
        ).toBeNull(),
      );
    },
  );
});
