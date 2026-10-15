import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import {
  ApiDocsSection,
  GoogleSection,
  PasswordSection,
} from "@/features/auth/components/SettingsSections";
import { openDocsSession } from "@/features/auth/api";
import { passwordSignInQuery } from "@/features/organization/api";
import type { PublicOrganization } from "@/lib/organization";
import { BASE_URL } from "@/lib/api/client";
import { toast } from "@/components/ui/sonner";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import "@/lib/i18n";

const BASE = {
  id: "018f0000-0000-7000-8000-0000000000a2",
  email: "an@example.com",
  fullName: "Nguyễn Văn An",
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
  permissions: ["learning.take_tests" as const],
  workspaces: ["app" as const],
};

function signedIn(over: { hasPassword: boolean; google: boolean }) {
  useAuthStore.getState().setSession("token", {
    ...BASE,
    hasPassword: over.hasPassword,
    linkedProviders: over.google ? ["google" as const] : [],
  });
}

const ORGANIZATION: PublicOrganization = {
  name: "Quizzivy",
  shortName: null,
  website: null,
  contactEmail: null,
  frontDeskPhone: null,
  frontDeskHours: null,
  address: null,
  logo: { lightUrl: null, darkUrl: null },
  signIn: { google: true, password: true },
  joinCodes: true,
};

function renderGoogle(organization?: PublicOrganization) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (organization) client.setQueryData(passwordSignInQuery().queryKey, organization);
  return render(
    <QueryClientProvider client={client}>
      <GoogleSection />
    </QueryClientProvider>,
  );
}

vi.mock("@/features/auth/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/features/auth/api")>()),
  openDocsSession: vi.fn(),
}));
vi.mock("@/components/ui/sonner", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/components/ui/sonner")>()),
  toast: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(toast).mockReset();
});
afterEach(() => useAuthStore.getState().clearSession());

/**
 * S-17 and S-10 write a sentence under every control on this screen; each of
 * the three sign-in states gets its own, and the app used to leave two of them
 * unsaid.
 */
describe("the settings cards say what the deck writes", () => {
  it("states the password rule under the new password (S-17)", () => {
    signedIn({ hasPassword: true, google: false });
    render(<PasswordSection />);

    const field = screen.getByLabelText("Mật khẩu mới");
    expect(
      screen.getByText("Ít nhất 8 ký tự, có số hoặc ký hiệu."),
    ).toBeInTheDocument();
    expect(field).toHaveAccessibleDescription("Ít nhất 8 ký tự, có số hoặc ký hiệu.");
  });

  it("says when the password was last changed, and nothing when it never was", () => {
    signedIn({ hasPassword: true, google: false });
    const { unmount } = render(<PasswordSection />);
    expect(screen.queryByText(/Đổi lần cuối/)).toBeNull();
    unmount();

    useAuthStore.setState((state) => ({
      user: state.user && {
        ...state.user,
        passwordChangedAt: new Date(Date.now() - 3 * 3_600_000).toISOString(),
      },
    }));
    render(<PasswordSection />);
    expect(screen.getByText("Đổi lần cuối 3 giờ trước.")).toBeInTheDocument();
  });

  it("says what unlinking would leave behind when both ways in exist (S-17)", () => {
    signedIn({ hasPassword: true, google: true });
    renderGoogle();

    expect(screen.getByText("Đã liên kết với Google.")).toBeInTheDocument();
    expect(screen.getByText(/Bỏ liên kết thì chỉ còn mật khẩu\./)).toBeInTheDocument();
    const unlink = screen.getByRole("button", { name: "Bỏ liên kết Google" });
    expect(unlink).not.toHaveAttribute("aria-disabled");
    expect(unlink).toHaveAccessibleDescription(/Bỏ liên kết thì chỉ còn mật khẩu/);
  });

  it("says a Google-only account has no password yet, and refuses the unlink while keeping it in the tab order (S-10)", async () => {
    signedIn({ hasPassword: false, google: true });
    renderGoogle();
    const fact =
      "Tài khoản này chưa có mật khẩu, nên chưa thể bỏ liên kết Google. Hãy đặt mật khẩu để có thêm một cách đăng nhập.";

    expect(screen.getByText(fact)).toBeInTheDocument();
    const unlink = screen.getByRole("button", { name: "Bỏ liên kết Google" });
    expect(unlink).toHaveAttribute("aria-disabled", "true");
    expect(unlink).toHaveAccessibleDescription(fact);
    expect(unlink).not.toBeDisabled();
    await userEvent.setup().click(unlink);
    expect(unlink).toHaveFocus();
    expect(screen.getByText("Đã liên kết với Google.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Gửi liên kết đặt mật khẩu qua email" }),
    ).toBeInTheDocument();
  });

  it("explains nothing about unlinking when there is nothing linked", () => {
    signedIn({ hasPassword: true, google: false });
    renderGoogle();

    expect(screen.getByText("Chưa liên kết với Google.")).toBeInTheDocument();
    expect(screen.queryByText(/Bỏ liên kết thì chỉ còn/)).toBeNull();
    expect(screen.queryByText(/cách duy nhất/)).toBeNull();
  });
});

describe("asking for a set-password link from the Google card", () => {
  const SEND = "Gửi liên kết đặt mật khẩu qua email";
  const SENT =
    "Đã gửi liên kết tới an@example.com. Liên kết chỉ dùng được một lần và hết hạn sau 24 giờ.";

  function requestAnswers(answer: () => Response | Promise<Response>) {
    const calls: Request[] = [];
    server.use(
      http.post(`${BASE_URL}/auth/set-password/request`, ({ request }) => {
        calls.push(request);
        return answer();
      }),
    );
    return calls;
  }

  function refusal(
    code: "PASSWORD_SIGN_IN_DISABLED" | "RATE_LIMITED" | "INTERNAL",
    message: string,
  ) {
    return {
      error: { code, message, requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e" },
    };
  }

  it("sends one request, says so while it is on its way, then names the address", async () => {
    let release: () => void = () => undefined;
    const calls = requestAnswers(
      () =>
        new Promise<Response>((resolve) => {
          release = () => resolve(new Response(null, { status: 204 }));
        }),
    );
    signedIn({ hasPassword: false, google: true });
    const user = userEvent.setup();
    renderGoogle();

    const send = screen.getByRole("button", { name: SEND });
    expect(send).not.toHaveAttribute("aria-disabled");
    await user.click(send);
    const sending = screen.getByRole("button", { name: "Đang gửi…" });
    expect(sending).toBe(send);
    expect(sending).toHaveAttribute("aria-disabled", "true");
    expect(sending).toHaveAttribute("aria-busy", "true");
    expect(sending).not.toBeDisabled();
    await user.click(sending);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.headers.get("Authorization")).toBe("Bearer token");

    release();
    expect(await screen.findByRole("status")).toHaveTextContent(SENT);
    const again = screen.getByRole("button", { name: "Gửi lại liên kết" });
    expect(again).toBe(send);
    expect(again).not.toHaveAttribute("aria-disabled");
    expect(screen.getByRole("button", { name: "Bỏ liên kết Google" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });

  it("reaches the button and Unlink from the keyboard, Unlink first", async () => {
    requestAnswers(() => new Response(null, { status: 204 }));
    signedIn({ hasPassword: false, google: true });
    const user = userEvent.setup();
    renderGoogle();

    await user.tab();
    expect(screen.getByRole("button", { name: "Bỏ liên kết Google" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: SEND })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(await screen.findByRole("status")).toHaveTextContent(SENT);
    expect(screen.getByRole("button", { name: "Gửi lại liên kết" })).toHaveFocus();
  });

  it.each([
    [
      "a 501",
      () =>
        contractJson(
          "/auth/set-password/request",
          "post",
          501,
          refusal("INTERNAL", "Máy chủ này chưa gửi được liên kết."),
        ),
      "Máy chủ này chưa gửi được liên kết.",
    ],
    [
      "a 422",
      () =>
        contractJson(
          "/auth/set-password/request",
          "post",
          422,
          refusal(
            "PASSWORD_SIGN_IN_DISABLED",
            "Trung tâm đã tắt đăng nhập bằng mật khẩu.",
          ),
        ),
      "Trung tâm đã tắt đăng nhập bằng mật khẩu.",
    ],
    [
      "a 429",
      () =>
        contractJson(
          "/auth/set-password/request",
          "post",
          429,
          refusal("RATE_LIMITED", "Bạn thử quá nhiều lần. Hãy chờ một phút."),
        ),
      "Bạn thử quá nhiều lần. Hãy chờ một phút.",
    ],
  ])(
    "shows the server's own words for %s, and lets the person try again",
    async (_, answer, message) => {
      const calls = requestAnswers(answer);
      signedIn({ hasPassword: false, google: true });
      const user = userEvent.setup();
      renderGoogle();

      await user.click(screen.getByRole("button", { name: SEND }));
      expect(await screen.findByRole("alert")).toHaveTextContent(message);
      expect(screen.queryByRole("status")).toBeNull();
      await user.click(screen.getByRole("button", { name: SEND }));
      await waitFor(() => expect(calls).toHaveLength(2));
    },
  );

  it("offers nothing, and says nothing of a password, while the centre is known to have password sign-in off", () => {
    signedIn({ hasPassword: false, google: true });
    renderGoogle({ ...ORGANIZATION, signIn: { google: true, password: false } });

    const fact = "Bạn chỉ đăng nhập bằng Google, nên chưa thể bỏ liên kết Google.";
    expect(screen.getByText(fact)).toBeInTheDocument();
    expect(screen.queryByText(/mật khẩu/i)).toBeNull();
    expect(screen.queryByRole("button", { name: SEND })).toBeNull();
    expect(
      screen.getByRole("button", { name: "Bỏ liên kết Google" }),
    ).toHaveAccessibleDescription(fact);
  });

  it("withdraws the link and the sentence once the centre answers that passwords are off, with nothing cached before", async () => {
    server.use(
      http.get(`${BASE_URL}/public/organization`, () =>
        contractJson("/public/organization", "get", 200, {
          ...ORGANIZATION,
          signIn: { google: true, password: false },
        }),
      ),
    );
    signedIn({ hasPassword: false, google: true });
    renderGoogle();

    expect(
      await screen.findByText(
        "Bạn chỉ đăng nhập bằng Google, nên chưa thể bỏ liên kết Google.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/mật khẩu/i)).toBeNull();
    expect(screen.queryByRole("button", { name: SEND })).toBeNull();
  });

  it("offers the link when the centre's answer says passwords are on", () => {
    signedIn({ hasPassword: false, google: true });
    renderGoogle(ORGANIZATION);
    expect(screen.getByRole("button", { name: SEND })).toBeInTheDocument();
  });

  it("offers no link to an account that already has a password", () => {
    signedIn({ hasPassword: true, google: true });
    renderGoogle();
    expect(screen.queryByRole("button", { name: SEND })).toBeNull();
  });
});

describe("the API reference entry", () => {
  function fakeTab() {
    return { opener: {} as unknown, location: { href: "about:blank" }, close: vi.fn() };
  }

  it("opens the tab in the click itself, cuts it off from this page, then points it at the docs", async () => {
    const tab = fakeTab();
    const open = vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    let finish: () => void = () => undefined;
    vi.mocked(openDocsSession).mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    render(<ApiDocsSection />);

    fireEvent.click(screen.getByRole("button", { name: "Mở tài liệu API" }));
    expect(open).toHaveBeenCalledWith("about:blank", "_blank");
    expect(tab.opener).toBeNull();
    expect(openDocsSession).toHaveBeenCalledOnce();
    expect(tab.location.href).toBe("about:blank");

    finish();
    await waitFor(() => expect(tab.location.href).toBe(`${BASE_URL}/docs`));
    open.mockRestore();
  });

  it("closes the tab and says so when the session cannot be opened", async () => {
    const tab = fakeTab();
    const open = vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    vi.mocked(openDocsSession).mockRejectedValue(new Error("offline"));
    const user = userEvent.setup();
    render(<ApiDocsSection />);

    await user.click(screen.getByRole("button", { name: "Mở tài liệu API" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không mở được tài liệu API.",
    );
    expect(tab.close).toHaveBeenCalledOnce();
    open.mockRestore();
  });

  it("asks for pop-ups instead of opening a session when the browser blocks the tab", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    vi.mocked(openDocsSession).mockReset();
    const user = userEvent.setup();
    render(<ApiDocsSection />);

    await user.click(screen.getByRole("button", { name: "Mở tài liệu API" }));

    expect(screen.getByRole("alert")).toHaveTextContent("Trình duyệt đã chặn tab mới.");
    expect(openDocsSession).not.toHaveBeenCalled();
    open.mockRestore();
  });
});
