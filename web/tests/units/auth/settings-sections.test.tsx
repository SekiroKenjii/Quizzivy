import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  ApiDocsSection,
  GoogleSection,
  LanguageSection,
  PasswordSection,
  ProfileSection,
} from "@/features/auth/components/SettingsSections";
import { openDocsSession, updateProfile } from "@/features/auth/api";
import { BASE_URL } from "@/lib/api/client";
import { toast } from "@/components/ui/sonner";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = {
  id: "018f0000-0000-7000-8000-0000000000a2",
  email: "an@example.com",
  fullName: "Nguyễn Văn An",
  role: "student" as const,
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
};

function signedIn(over: { hasPassword: boolean; google: boolean }) {
  useAuthStore.getState().setSession("token", {
    ...BASE,
    hasPassword: over.hasPassword,
    linkedProviders: over.google ? ["google" as const] : [],
  });
}

vi.mock("@/features/auth/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/features/auth/api")>()),
  updateProfile: vi.fn(),
  openDocsSession: vi.fn(),
}));
vi.mock("@/components/ui/sonner", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/components/ui/sonner")>()),
  toast: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(updateProfile).mockReset();
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

  it("says what unlinking would leave behind when both ways in exist (S-17)", () => {
    signedIn({ hasPassword: true, google: true });
    render(<GoogleSection />);

    expect(screen.getByText("Đã liên kết với Google.")).toBeInTheDocument();
    expect(screen.getByText(/Bỏ liên kết thì chỉ còn mật khẩu\./)).toBeInTheDocument();
    const unlink = screen.getByRole("button", { name: "Bỏ liên kết Google" });
    expect(unlink).toHaveAttribute("aria-disabled", "false");
    expect(unlink).toHaveAccessibleDescription(/Bỏ liên kết thì chỉ còn mật khẩu/);
  });

  it("says Google is the only way in, and refuses the unlink, when there is no password (S-10)", () => {
    signedIn({ hasPassword: false, google: true });
    render(<GoogleSection />);

    expect(
      screen.getByText(/Google đang là cách duy nhất để đăng nhập/),
    ).toBeInTheDocument();
    // aria-disabled, not disabled: S-10 keeps it focusable so the reason beside
    // it is announced rather than silently skipped.
    const unlink = screen.getByRole("button", { name: "Bỏ liên kết Google" });
    expect(unlink).toHaveAttribute("aria-disabled", "true");
    expect(unlink).toHaveAccessibleDescription(/cách duy nhất để đăng nhập/);
    expect(unlink).not.toBeDisabled();
  });

  it("explains nothing about unlinking when there is nothing linked", () => {
    signedIn({ hasPassword: true, google: false });
    render(<GoogleSection />);

    expect(screen.getByText("Chưa liên kết với Google.")).toBeInTheDocument();
    expect(screen.queryByText(/Bỏ liên kết thì chỉ còn/)).toBeNull();
    expect(screen.queryByText(/cách duy nhất/)).toBeNull();
  });

  it("says what the language switch does and what it leaves alone (S-17)", () => {
    render(<LanguageSection />);

    expect(
      screen.getByText(
        "Đổi ngôn ngữ giao diện. Đề thi vẫn hiện đúng như giáo viên soạn.",
      ),
    ).toBeInTheDocument();
  });
});

/**
 * The "Hồ sơ" card, which both settings boards draw and #94 asked for: the one
 * thing an account may change about itself, beside the one it may not.
 */
describe("the profile card", () => {
  it("edits the name, keeps the email read-only, and says where the email came from", async () => {
    signedIn({ hasPassword: true, google: true });
    vi.mocked(updateProfile).mockResolvedValue({
      ...BASE,
      fullName: "Nguyễn Đức Minh",
      hasPassword: true,
      linkedProviders: ["google"],
    });
    const user = userEvent.setup();
    render(<ProfileSection />);

    const email = screen.getByLabelText("Email");
    expect(email).toHaveValue("an@example.com");
    expect(email).toBeDisabled();
    expect(email).toHaveAccessibleDescription(
      "Email lấy từ tài khoản Google, không đổi được.",
    );

    const name = screen.getByLabelText("Họ và tên");
    expect(name).toHaveValue("Nguyễn Văn An");
    // Nothing to save until something changes.
    expect(screen.getByRole("button", { name: "Lưu thay đổi" })).toBeDisabled();

    await user.clear(name);
    await user.type(name, "Nguyễn Đức Minh");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() => expect(updateProfile).toHaveBeenCalledWith("Nguyễn Đức Minh"));
    // F-08 confirms a completed action with a toast, not a sentence that stays.
    await waitFor(() => expect(toast).toHaveBeenCalledWith("Đã lưu họ và tên."));
    expect(useAuthStore.getState().user?.fullName).toBe("Nguyễn Đức Minh");
  });

  it("names the teacher as the source when there is no Google account", () => {
    signedIn({ hasPassword: true, google: false });
    render(<ProfileSection />);

    expect(screen.getByLabelText("Email")).toHaveAccessibleDescription(
      "Email do giáo viên cấp, không đổi được.",
    );
  });

  it("refuses an empty name without asking the server", async () => {
    signedIn({ hasPassword: true, google: false });
    const user = userEvent.setup();
    render(<ProfileSection />);

    await user.clear(screen.getByLabelText("Họ và tên"));
    await user.type(screen.getByLabelText("Họ và tên"), " ");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    expect(
      await screen.findByText("Họ và tên không được để trống."),
    ).toBeInTheDocument();
    expect(updateProfile).not.toHaveBeenCalled();
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
