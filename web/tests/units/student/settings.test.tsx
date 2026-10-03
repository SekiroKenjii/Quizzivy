import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import StudentLayout from "@/layouts/StudentLayout";
import StudentSettingsPage from "@/features/auth/pages/StudentSettingsPage";
import type { User } from "@/features/auth/api";
import i18n, { setLocale } from "@/lib/i18n";
import { readLargerTestText, writeLargerTestText } from "@/lib/testText";
import { readThemePreference, writeThemePreference } from "@/lib/theme";
import { notify } from "@/lib/toast";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import { BASE, STUDENT } from "./support";

const flags = vi.hoisted(() => ({
  notifications: false,
  messages: false,
  schedule: false,
  grades: false,
  learn: false,
}));
vi.mock("@/app/modules", () => ({ modules: flags }));

const google = vi.hoisted(() => ({
  available: true,
  error: null as string | null,
  pending: false,
  start: vi.fn(),
}));
vi.mock("@/features/auth/google/useGoogleSignIn", () => ({
  googleSignInAvailable: () => google.available,
  useGoogleSignIn: () => ({
    start: google.start,
    error: google.error,
    pending: google.pending,
  }),
}));

vi.mock("@/lib/toast", () => ({
  notify: { success: vi.fn(), info: vi.fn(), warning: vi.fn(), error: vi.fn() },
}));

const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const NAV = "Điều hướng chính";
const SWITCHER = "Mục cài đặt";

function account(over: Partial<User> = {}): User {
  return { ...STUDENT, ...over };
}

function open(path = "/app/settings", over: Partial<User> = {}) {
  useAuthStore.getState().setSession("token", account(over));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        element: <StudentLayout />,
        children: [
          {
            path: "/app/settings/:section?",
            handle: { detail: { titleKey: "nav.settings", back: "/app" } },
            element: <StudentSettingsPage />,
          },
        ],
      },
    ],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

function failure(code: string, message: string) {
  return { error: { code, message, requestId: REQUEST_ID } };
}

function profileAnswers(status: 200 | 400 = 200) {
  const bodies: unknown[] = [];
  server.use(
    http.patch(`${BASE}/auth/me`, async ({ request }) => {
      const body = (await request.json()) as { fullName: string };
      bodies.push(body);
      if (status === 400)
        return contractJson(
          "/auth/me",
          "patch",
          400,
          failure("VALIDATION_FAILED", "Họ và tên không hợp lệ."),
        );
      return contractJson(
        "/auth/me",
        "patch",
        200,
        account({ fullName: body.fullName.trim() }),
      );
    }),
  );
  return bodies;
}

function passwordAnswers(refusal?: { code: string; message: string }) {
  const bodies: unknown[] = [];
  server.use(
    http.post(`${BASE}/auth/change-password`, async ({ request }) => {
      bodies.push(await request.json());
      if (refusal === undefined) return new Response(null, { status: 204 });
      return contractJson(
        "/auth/change-password",
        "post",
        400,
        failure(refusal.code, refusal.message),
      );
    }),
  );
  return bodies;
}

function held() {
  const waiting: (() => void)[] = [];
  return {
    wait: () => new Promise<void>((resolve) => waiting.push(resolve)),
    release: () => {
      for (const resolve of waiting.splice(0)) resolve();
    },
  };
}

function meServes(user: User) {
  const calls: string[] = [];
  server.use(
    http.get(`${BASE}/auth/me`, () => {
      calls.push("me");
      return contractJson("/auth/me", "get", 200, user);
    }),
  );
  return calls;
}

function switcher() {
  return within(screen.getByRole("group", { name: SWITCHER }));
}

function meter() {
  const bars = [...screen.getByTestId("password-meter").children];
  return {
    filled: bars.filter((bar) => bar.hasAttribute("data-filled")).length,
    tones: [
      ...new Set(
        bars
          .filter((bar) => bar.hasAttribute("data-filled"))
          .map((bar) => /bg-(danger|warning|success)/.exec(bar.className)?.[1]),
      ),
    ],
  };
}

async function openPasswordForm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Đổi" }));
  return {
    current: screen.getByLabelText("Mật khẩu hiện tại"),
    next: screen.getByLabelText("Mật khẩu mới"),
    submit: screen.getByRole("button", { name: "Cập nhật mật khẩu" }),
  };
}

beforeEach(() => {
  viewport("desktop");
  localStorage.clear();
  google.available = true;
  google.error = null;
  google.pending = false;
  google.start.mockReset();
  vi.mocked(notify.success).mockReset();
  server.use(
    http.get(`${BASE}/app/assignments`, () =>
      contractJson("/app/assignments", "get", 200, {
        dueNow: [],
        upcoming: [],
        completed: [],
      }),
    ),
    http.get(`${BASE}/app/classes`, () =>
      contractJson("/app/classes", "get", 200, { items: [] }),
    ),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  useAuthStore.getState().clearSession();
  writeThemePreference("light");
  writeLargerTestText(false);
  setLocale("vi");
  localStorage.clear();
});

describe("the settings page", () => {
  it("is a 760 column with its heading and the three sections that exist, Profile first", async () => {
    open();
    const heading = await screen.findByRole("heading", { level: 1, name: "Cài đặt" });
    expect(heading).not.toHaveClass("sr-only");
    expect(heading.parentElement).toHaveClass("max-w-190", "mx-auto");

    expect(
      switcher()
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Hồ sơ", "Đăng nhập", "Giao diện"]);
    expect(switcher().getByRole("button", { name: "Hồ sơ" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("region", { name: "Hồ sơ" })).toBeVisible();
    expect(screen.queryByRole("region", { name: "Đăng nhập" })).toBeNull();
    expect(screen.queryByRole("region", { name: "Giao diện" })).toBeNull();
  });

  it("lights no destination in the top bar", async () => {
    open();
    const nav = within(await screen.findByRole("navigation", { name: NAV }));
    for (const link of nav.getAllByRole("link")) {
      expect(link).not.toHaveAttribute("aria-current");
    }
  });

  it("leaves the title to the phone header and keeps its own heading for screen readers", async () => {
    viewport("phone");
    open();
    const heading = await screen.findByRole("heading", { level: 1, name: "Cài đặt" });
    expect(heading).toHaveClass("sr-only");
    expect(
      within(screen.getAllByRole("banner")[0]!).getByText("Cài đặt"),
    ).toBeInTheDocument();
    expect(screen.getByRole("group", { name: SWITCHER })).toBeInTheDocument();
  });

  it("moves between its sections by URL and shows one at a time", async () => {
    const user = userEvent.setup();
    const router = open();
    await screen.findByRole("heading", { level: 1 });

    await user.click(switcher().getByRole("button", { name: "Đăng nhập" }));
    expect(router.state.location.pathname).toBe("/app/settings/sign-in");
    expect(screen.getByRole("region", { name: "Đăng nhập" })).toBeVisible();
    expect(screen.queryByRole("region", { name: "Hồ sơ" })).toBeNull();
    expect(switcher().getByRole("button", { name: "Đăng nhập" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(switcher().getByRole("button", { name: "Hồ sơ" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );

    await user.click(switcher().getByRole("button", { name: "Giao diện" }));
    expect(router.state.location.pathname).toBe("/app/settings/appearance");
    expect(screen.getByRole("region", { name: "Giao diện" })).toBeVisible();
    expect(screen.queryByRole("region", { name: "Đăng nhập" })).toBeNull();

    await user.click(switcher().getByRole("button", { name: "Hồ sơ" }));
    expect(router.state.location.pathname).toBe("/app/settings");
    expect(screen.getByRole("region", { name: "Hồ sơ" })).toBeVisible();
  });

  it.each([
    ["/app/settings/profile", "Hồ sơ"],
    ["/app/settings/sign-in", "Đăng nhập"],
    ["/app/settings/appearance", "Giao diện"],
  ])("opens %s on its own section and stays there", async (path, section) => {
    const router = open(path);
    expect(await screen.findByRole("region", { name: section })).toBeVisible();
    expect(router.state.location.pathname).toBe(path);
    expect(switcher().getByRole("button", { name: section })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it.each([
    ["security", "/app/settings/sign-in", "Đăng nhập"],
    ["preferences", "/app/settings", "Hồ sơ"],
    ["notifications", "/app/settings", "Hồ sơ"],
    ["constructor", "/app/settings", "Hồ sơ"],
    ["khong-co", "/app/settings", "Hồ sơ"],
  ])("redirects the slug %s to %s", async (slug, path, section) => {
    const router = open(`/app/settings/${slug}`);
    expect(await screen.findByRole("region", { name: section })).toBeVisible();
    await waitFor(() => expect(router.state.location.pathname).toBe(path));
    expect(router.state.historyAction).toBe("REPLACE");
  });

  it("has no notifications section while the module has not shipped", async () => {
    open();
    await screen.findByRole("heading", { level: 1 });
    expect(switcher().queryByRole("button", { name: "Thông báo" })).toBeNull();
    expect(screen.queryByRole("switch", { name: /bài|kết quả/i })).toBeNull();
  });

  it("draws no sign-out and no photo control: the account menu signs out", async () => {
    const user = userEvent.setup();
    open();
    const main = within(await screen.findByRole("main"));
    expect(main.queryByRole("button", { name: "Đăng xuất" })).toBeNull();
    expect(main.queryByRole("button", { name: /ảnh/i })).toBeNull();

    await user.click(
      screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" }),
    );
    expect(await screen.findByRole("menuitem", { name: "Đăng xuất" })).toBeVisible();
  });
});

describe("profile", () => {
  it("shows the initials, the name, the email it cannot change and the language", async () => {
    open();
    const card = within(await screen.findByRole("region", { name: "Hồ sơ" }));
    expect(card.getByText("AN")).toBeInTheDocument();

    const name = card.getByLabelText("Họ và tên");
    expect(name).toHaveValue("Nguyễn Văn An");
    expect(name).toHaveAccessibleDescription("Giáo viên của bạn thấy tên này.");

    const email = card.getByLabelText("Email");
    expect(email).toHaveValue("an@example.com");
    expect(email).toBeDisabled();
    expect(email).toHaveAccessibleDescription(
      "Muốn đổi email, hãy nhờ giáo viên của bạn.",
    );

    expect(card.getByRole("combobox", { name: "Ngôn ngữ" })).toHaveTextContent(
      "Tiếng Việt",
    );
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(screen.queryByRole("button", { name: "Lưu thay đổi" })).toBeNull();
  });

  it("offers to save only once the name differs, and saves it through PATCH /auth/me", async () => {
    const user = userEvent.setup();
    const bodies = profileAnswers();
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.clear(name);
    await user.type(name, "Nguyễn Đức Minh");
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    expect(bodies).toEqual([]);

    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    await waitFor(() => expect(bodies).toEqual([{ fullName: "Nguyễn Đức Minh" }]));
    await waitFor(() =>
      expect(notify.success).toHaveBeenCalledWith("Đã lưu họ và tên."),
    );
    expect(useAuthStore.getState().user?.fullName).toBe("Nguyễn Đức Minh");
    expect(name).toHaveValue("Nguyễn Đức Minh");
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(screen.queryByRole("button", { name: "Lưu thay đổi" })).toBeNull();
    expect(
      within(screen.getByRole("region", { name: "Hồ sơ" })).getByText("MN"),
    ).toBeInTheDocument();
  });

  it("leaves the bar once a name that only gained spaces is saved", async () => {
    const user = userEvent.setup();
    const bodies = profileAnswers();
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.type(name, "  ");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    await waitFor(() => expect(bodies).toEqual([{ fullName: "Nguyễn Văn An" }]));
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Lưu thay đổi" })).toBeNull(),
    );
    expect(name).toHaveValue("Nguyễn Văn An");
  });

  it("discards a change and asks the server nothing", async () => {
    const user = userEvent.setup();
    const bodies = profileAnswers();
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.type(name, " Bình");
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));

    expect(name).toHaveValue("Nguyễn Văn An");
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(bodies).toEqual([]);
  });

  it("sends nothing when Enter is pressed on an unchanged name", async () => {
    const user = userEvent.setup();
    const bodies = profileAnswers();
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.click(name);
    await user.keyboard("{Enter}");
    fireEvent.submit(name.closest("form")!);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(bodies).toEqual([]);
  });

  it("refuses an empty name under the field without asking the server", async () => {
    const user = userEvent.setup();
    const bodies = profileAnswers();
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.clear(name);
    await user.type(name, " ");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Họ và tên không được để trống.",
    );
    expect(name).toBeInvalid();
    expect(name).toHaveAccessibleDescription("Họ và tên không được để trống.");
    expect(screen.queryByText("Giáo viên của bạn thấy tên này.")).toBeNull();
    expect(bodies).toEqual([]);
  });

  it("refuses a name longer than 200 characters", async () => {
    const user = userEvent.setup();
    const bodies = profileAnswers();
    open();
    const name = await screen.findByLabelText("Họ và tên");

    fireEvent.change(name, { target: { value: "a".repeat(201) } });
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Họ và tên tối đa 200 ký tự.",
    );
    expect(bodies).toEqual([]);

    fireEvent.change(name, { target: { value: "a".repeat(200) } });
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    await waitFor(() => expect(bodies).toHaveLength(1));
  });

  it("keeps the change and says why when the save fails, then saves on a second try", async () => {
    const user = userEvent.setup();
    profileAnswers(400);
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.type(name, " Bình");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Họ và tên không hợp lệ.",
    );
    expect(screen.queryByText("Bạn có thay đổi chưa lưu.")).toBeNull();
    expect(name).toHaveValue("Nguyễn Văn An Bình");
    expect(notify.success).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user?.fullName).toBe("Nguyễn Văn An");

    const bodies = profileAnswers();
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    await waitFor(() => expect(bodies).toEqual([{ fullName: "Nguyễn Văn An Bình" }]));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(useAuthStore.getState().user?.fullName).toBe("Nguyễn Văn An Bình");
  });

  it("sends the name once however often the form is submitted while it saves", async () => {
    const user = userEvent.setup();
    const gate = held();
    const bodies: unknown[] = [];
    server.use(
      http.patch(`${BASE}/auth/me`, async ({ request }) => {
        bodies.push(await request.json());
        await gate.wait();
        return contractJson(
          "/auth/me",
          "patch",
          200,
          account({ fullName: "Nguyễn Văn An Bình" }),
        );
      }),
    );
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.type(name, " Bình");
    const sheet = name.closest("form")!;
    fireEvent.submit(sheet);
    await waitFor(() => expect(bodies).toHaveLength(1));
    const save = screen.getByRole("button", { name: "Đang lưu…" });
    expect(save).toBeDisabled();
    expect(screen.getByRole("button", { name: "Bỏ thay đổi" })).toBeDisabled();

    fireEvent.submit(sheet);
    await new Promise((resolve) => setTimeout(resolve, 20));
    gate.release();

    await waitFor(() => expect(notify.success).toHaveBeenCalledOnce());
    expect(bodies).toHaveLength(1);
  });

  it("forgets a failed save's reason when the change is discarded", async () => {
    const user = userEvent.setup();
    profileAnswers(400);
    open();
    const name = await screen.findByLabelText("Họ và tên");

    await user.type(name, " Bình");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));
    await user.type(name, " Chi");

    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("switches the language at once, stores it, and is never an unsaved change", async () => {
    const user = userEvent.setup();
    open();
    const trigger = await screen.findByRole("combobox", { name: "Ngôn ngữ" });

    await user.click(trigger);
    const list = await screen.findByRole("listbox");
    expect(
      within(list)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["Tiếng Việt", "English"]);
    await user.click(within(list).getByRole("option", { name: "English" }));

    expect(i18n.language).toBe("en");
    expect(localStorage.getItem("quizzivy.locale")).toBe("en");
    expect(document.documentElement.lang).toBe("en");
    expect(await screen.findByLabelText("Full name")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Language" })).toHaveTextContent(
      "English",
    );
    expect(screen.queryByRole("button", { name: "Save changes" })).toBeNull();
  });
});

describe("sign-in: the password", () => {
  it("draws the row closed, with no line about when it changed", async () => {
    open("/app/settings/sign-in");
    const card = within(await screen.findByRole("region", { name: "Đăng nhập" }));
    expect(card.getByText("Mật khẩu")).toBeInTheDocument();

    const change = card.getByRole("button", { name: "Đổi" });
    expect(change).toHaveAttribute("aria-expanded", "false");
    expect(change).toHaveAccessibleDescription("Mật khẩu");
    expect(card.queryByLabelText("Mật khẩu mới")).toBeNull();
    expect(card.queryByText(/Đã đổi|trước/)).toBeNull();
  });

  it("opens both fields with an empty meter, the rule and a button that waits", async () => {
    const user = userEvent.setup();
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    expect(form.current).toHaveAttribute("type", "password");
    expect(form.current).toHaveAttribute("autocomplete", "current-password");
    expect(form.next).toHaveAttribute("autocomplete", "new-password");
    expect(form.next).toHaveAccessibleDescription("Ít nhất 8 ký tự.");
    expect(screen.getByTestId("password-meter").children).toHaveLength(4);
    expect(meter().filled).toBe(0);
    expect(form.submit).toHaveAttribute("aria-disabled", "true");
    expect(form.submit).not.toBeDisabled();
    expect(screen.getByRole("button", { name: "Huỷ" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    expect(screen.queryByRole("button", { name: "Đổi" })).toBeNull();
  });

  it.each([
    ["abc", 1, "danger", "Ít nhất 8 ký tự."],
    ["abc1", 2, "warning", "Ít nhất 8 ký tự."],
    ["abcdefgh", 2, "warning", "Tạm được. Thêm số hoặc ký hiệu để mật khẩu mạnh hơn."],
    [
      "abcdefghijkl",
      2,
      "warning",
      "Tạm được. Thêm số hoặc ký hiệu để mật khẩu mạnh hơn.",
    ],
    ["abcdefg1", 3, "success", "Mạnh."],
    ["abcdefghijk!", 4, "success", "Rất mạnh."],
  ])("scores %s at %i bars in %s and says so", async (typed, bars, tone, hint) => {
    const user = userEvent.setup();
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.next, typed);
    expect(meter()).toEqual({ filled: bars, tones: [tone] });
    expect(form.next).toHaveAccessibleDescription(hint);
    expect(form.submit).toHaveAttribute(
      "aria-disabled",
      [...typed].length >= 8 ? "false" : "true",
    );
  });

  it("does nothing while the new password is shorter than 8 characters", async () => {
    const user = userEvent.setup();
    const bodies = passwordAnswers();
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "ngan1!");
    await user.click(form.submit);

    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(bodies).toEqual([]);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(form.next).toHaveAccessibleDescription("Ít nhất 8 ký tự.");
  });

  it("treats Enter as the button: nothing while short, the change once long enough", async () => {
    const user = userEvent.setup();
    const bodies = passwordAnswers();
    meServes(account());
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "ngan1!{Enter}");
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(bodies).toEqual([]);
    expect(screen.queryByRole("alert")).toBeNull();

    await user.type(form.next, "them{Enter}");
    await waitFor(() =>
      expect(bodies).toEqual([
        { currentPassword: "mat-khau-cu-1", newPassword: "ngan1!them" },
      ]),
    );
    await waitFor(() => expect(notify.success).toHaveBeenCalledOnce());
  });

  it("lets the form's rule, not the meter, refuse a password with no number or symbol", async () => {
    const user = userEvent.setup();
    const bodies = passwordAnswers();
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "chicochu");
    await user.click(form.submit);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Mật khẩu mới cần có số hoặc ký hiệu.",
    );
    expect(form.next).toHaveAccessibleDescription(
      "Mật khẩu mới cần có số hoặc ký hiệu.",
    );
    expect(bodies).toEqual([]);

    await user.type(form.next, "9");
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(form.next).toHaveAccessibleDescription("Mạnh.");
  });

  it("changes the password, reads the user again, closes the form and says so", async () => {
    const user = userEvent.setup();
    const bodies = passwordAnswers();
    const reads = meServes(account());
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "mat-khau-moi-2");
    await user.click(form.submit);

    await waitFor(() =>
      expect(bodies).toEqual([
        { currentPassword: "mat-khau-cu-1", newPassword: "mat-khau-moi-2" },
      ]),
    );
    await waitFor(() =>
      expect(notify.success).toHaveBeenCalledWith(
        "Đã cập nhật mật khẩu. Các thiết bị khác đã được đăng xuất.",
      ),
    );
    expect(reads).toEqual(["me"]);
    expect(screen.queryByLabelText("Mật khẩu mới")).toBeNull();
    expect(screen.getByRole("button", { name: "Đổi" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );

    const again = await openPasswordForm(user);
    expect(again.current).toHaveValue("");
    expect(again.next).toHaveValue("");
  });

  it("sends one request however often the form is submitted while it saves", async () => {
    const user = userEvent.setup();
    const gate = held();
    const bodies: unknown[] = [];
    server.use(
      http.post(`${BASE}/auth/change-password`, async ({ request }) => {
        bodies.push(await request.json());
        await gate.wait();
        return new Response(null, { status: 204 });
      }),
    );
    meServes(account());
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "mat-khau-moi-2");
    const sheet = form.next.closest("form")!;
    fireEvent.submit(sheet);
    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(form.submit).toHaveAttribute("aria-disabled", "true");
    expect(form.submit).toHaveTextContent("Đang lưu…");

    fireEvent.submit(sheet);
    await user.click(form.submit);
    await user.type(form.next, "{Enter}");
    await new Promise((resolve) => setTimeout(resolve, 20));
    gate.release();

    await waitFor(() => expect(notify.success).toHaveBeenCalledOnce());
    expect(bodies).toHaveLength(1);
  });

  it.each([
    [
      "PASSWORD_UNCHANGED",
      "The new password must differ.",
      "Mật khẩu mới phải khác mật khẩu hiện tại.",
    ],
    [
      "INVALID_CREDENTIALS",
      "Mật khẩu hiện tại không đúng.",
      "Mật khẩu hiện tại không đúng.",
    ],
  ])("keeps the form open with the reason for %s", async (code, message, shown) => {
    const user = userEvent.setup();
    const bodies = passwordAnswers({ code, message });
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "mat-khau-moi-2");
    await user.click(form.submit);

    expect(await screen.findByRole("alert")).toHaveTextContent(shown);
    expect(bodies).toHaveLength(1);
    expect(form.next).toHaveValue("mat-khau-moi-2");
    expect(notify.success).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Huỷ" })).toBeInTheDocument();
  });

  it("forgets what was typed, and a refusal, when the form is cancelled", async () => {
    const user = userEvent.setup();
    passwordAnswers({
      code: "INVALID_CREDENTIALS",
      message: "Mật khẩu hiện tại không đúng.",
    });
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "mat-khau-moi-2");
    await user.click(form.submit);
    await screen.findByRole("alert");

    await user.click(screen.getByRole("button", { name: "Huỷ" }));
    expect(screen.queryByLabelText("Mật khẩu mới")).toBeNull();

    const again = await openPasswordForm(user);
    expect(again.current).toHaveValue("");
    expect(again.next).toHaveValue("");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(meter().filled).toBe(0);
  });

  it("shows and hides what is typed", async () => {
    const user = userEvent.setup();
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    const form = await openPasswordForm(user);

    await user.type(form.next, "mat-khau-moi-2");
    await user.click(screen.getAllByRole("button", { name: "Hiện mật khẩu" })[1]!);
    expect(form.next).toHaveAttribute("type", "text");
    expect(form.current).toHaveAttribute("type", "password");
    await user.click(screen.getByRole("button", { name: "Ẩn mật khẩu" }));
    expect(form.next).toHaveAttribute("type", "password");
    expect(form.next).toHaveValue("mat-khau-moi-2");
  });
});

describe("sign-in: Google", () => {
  it("unlinks a linked account that has a password, then says how to sign in", async () => {
    const user = userEvent.setup();
    const calls: string[] = [];
    server.use(
      http.delete(`${BASE}/auth/google/link`, () => {
        calls.push("unlink");
        return new Response(null, { status: 204 });
      }),
    );
    const reads = meServes(account({ linkedProviders: [] }));
    open("/app/settings/sign-in", { linkedProviders: ["google"] });
    const card = within(await screen.findByRole("region", { name: "Đăng nhập" }));
    expect(card.getByText("Đã liên kết")).toBeInTheDocument();
    expect(card.queryByText(/an@example\.com/)).toBeNull();
    expect(card.queryByRole("button", { name: "Liên kết Google" })).toBeNull();

    const unlink = card.getByRole("button", { name: "Bỏ liên kết" });
    expect(unlink).toHaveAttribute("aria-disabled", "false");
    expect(unlink).toHaveAccessibleDescription("Google Đã liên kết");
    await user.click(unlink);

    await waitFor(() =>
      expect(notify.success).toHaveBeenCalledWith(
        "Đã bỏ liên kết Google. Hãy dùng mật khẩu để đăng nhập.",
      ),
    );
    expect(calls).toEqual(["unlink"]);
    expect(reads).toEqual(["me"]);
    expect(
      card.getByText("Chưa liên kết. Liên kết để đăng nhập bằng một chạm."),
    ).toBeInTheDocument();
    expect(card.getByRole("button", { name: "Liên kết Google" })).toBeInTheDocument();
    expect(card.queryByRole("button", { name: "Bỏ liên kết" })).toBeNull();
  });

  it("unlinks once however often the button is pressed while it works", async () => {
    const user = userEvent.setup();
    const gate = held();
    const calls: string[] = [];
    server.use(
      http.delete(`${BASE}/auth/google/link`, async () => {
        calls.push("unlink");
        await gate.wait();
        return new Response(null, { status: 204 });
      }),
    );
    meServes(account({ linkedProviders: [] }));
    open("/app/settings/sign-in", { linkedProviders: ["google"] });
    const card = within(await screen.findByRole("region", { name: "Đăng nhập" }));
    const unlink = card.getByRole("button", { name: "Bỏ liên kết" });

    await user.click(unlink);
    await waitFor(() => expect(calls).toHaveLength(1));
    expect(unlink).toHaveAttribute("aria-disabled", "true");
    await user.click(unlink);
    await new Promise((resolve) => setTimeout(resolve, 20));
    gate.release();

    await waitFor(() => expect(notify.success).toHaveBeenCalledOnce());
    expect(calls).toEqual(["unlink"]);
  });

  it("keeps Unlink switched off, with the reason, when Google is the only way in", async () => {
    const user = userEvent.setup();
    const calls: string[] = [];
    server.use(
      http.delete(`${BASE}/auth/google/link`, () => {
        calls.push("unlink");
        return new Response(null, { status: 204 });
      }),
    );
    open("/app/settings/sign-in", { hasPassword: false, linkedProviders: ["google"] });
    const card = within(await screen.findByRole("region", { name: "Đăng nhập" }));

    expect(card.getByText(/Google đang là cách duy nhất để đăng nhập/)).toBeVisible();
    const unlink = card.getByRole("button", { name: "Bỏ liên kết" });
    expect(unlink).toHaveAttribute("aria-disabled", "true");
    expect(unlink).not.toBeDisabled();
    expect(unlink).toHaveAccessibleDescription(/cách duy nhất để đăng nhập/);
    await user.click(unlink);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(calls).toEqual([]);

    expect(card.queryByText("Mật khẩu")).toBeNull();
    expect(card.queryByRole("button", { name: "Đổi" })).toBeNull();
  });

  it("starts the link through the redirect flow, never a script from Google", async () => {
    const user = userEvent.setup();
    open("/app/settings/sign-in");
    const card = within(await screen.findByRole("region", { name: "Đăng nhập" }));
    expect(
      card.getByText("Chưa liên kết. Liên kết để đăng nhập bằng một chạm."),
    ).toBeInTheDocument();
    expect(card.queryByRole("button", { name: "Bỏ liên kết" })).toBeNull();

    await user.click(card.getByRole("button", { name: "Liên kết Google" }));
    expect(google.start).toHaveBeenCalledExactlyOnceWith({
      mode: "link",
      next: window.location.pathname,
    });
    expect(document.querySelector("script[src*='accounts.google.com']")).toBeNull();
  });

  it("offers no link, and says why, where Google sign-in is not set up", async () => {
    google.available = false;
    open("/app/settings/sign-in");
    const card = within(await screen.findByRole("region", { name: "Đăng nhập" }));
    expect(card.getByText("Hiện chưa thể đăng nhập bằng Google.")).toBeInTheDocument();
    expect(card.queryByRole("button", { name: "Liên kết Google" })).toBeNull();
  });

  it("says so when the link cannot start", async () => {
    google.error = "Hiện chưa thể đăng nhập bằng Google.";
    open("/app/settings/sign-in");
    await screen.findByRole("region", { name: "Đăng nhập" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Hiện chưa thể đăng nhập bằng Google.",
    );
  });

  it("keeps the link and says why when the unlink is refused", async () => {
    const user = userEvent.setup();
    server.use(
      http.delete(`${BASE}/auth/google/link`, () =>
        contractJson(
          "/auth/google/link",
          "delete",
          409,
          failure("LAST_LOGIN_METHOD", "Tài khoản không còn cách đăng nhập nào khác."),
        ),
      ),
    );
    open("/app/settings/sign-in", { linkedProviders: ["google"] });
    const card = within(await screen.findByRole("region", { name: "Đăng nhập" }));

    await user.click(card.getByRole("button", { name: "Bỏ liên kết" }));
    expect(await card.findByRole("alert")).toHaveTextContent(
      "Tài khoản không còn cách đăng nhập nào khác.",
    );
    expect(card.getByText("Đã liên kết")).toBeInTheDocument();
    expect(notify.success).not.toHaveBeenCalled();
    expect(card.getByRole("button", { name: "Bỏ liên kết" })).toHaveAttribute(
      "aria-disabled",
      "false",
    );
  });
});

describe("appearance", () => {
  it("draws the three themes with the stored one on, and applies a choice at once", async () => {
    const user = userEvent.setup();
    open("/app/settings/appearance");
    const card = within(await screen.findByRole("region", { name: "Giao diện" }));
    const themes = within(card.getByRole("group", { name: "Chủ đề" }));
    expect(themes.getAllByRole("button").map((button) => button.textContent)).toEqual([
      "Sáng",
      "Tối",
      "Theo thiết bị",
    ]);
    expect(themes.getByRole("button", { name: "Sáng" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(themes.getByRole("button", { name: "Tối" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );

    await user.click(themes.getByRole("button", { name: "Tối" }));
    expect(readThemePreference()).toBe("dark");
    expect(localStorage.getItem("quizzivy.theme")).toBe("dark");
    expect(document.documentElement).toHaveClass("dark");
    expect(themes.getByRole("button", { name: "Tối" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(themes.getByRole("button", { name: "Sáng" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );

    await user.click(themes.getByRole("button", { name: "Sáng" }));
    expect(readThemePreference()).toBe("light");
    expect(document.documentElement).not.toHaveClass("dark");
  });

  it("follows the device when Device is chosen, and keeps Device on", async () => {
    vi.stubGlobal(
      "matchMedia",
      (query: string) =>
        ({
          matches: true,
          media: query,
          addEventListener: () => {},
          removeEventListener: () => {},
        }) as unknown as MediaQueryList,
    );
    const user = userEvent.setup();
    open("/app/settings/appearance");
    const themes = within(await screen.findByRole("group", { name: "Chủ đề" }));

    await user.click(themes.getByRole("button", { name: "Theo thiết bị" }));
    expect(readThemePreference()).toBe("system");
    expect(document.documentElement).toHaveClass("dark");
    expect(themes.getByRole("button", { name: "Theo thiết bị" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(themes.getByRole("button", { name: "Tối" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("follows the account menu's theme item", async () => {
    open("/app/settings/appearance");
    const themes = within(await screen.findByRole("group", { name: "Chủ đề" }));
    act(() => writeThemePreference("system"));
    expect(themes.getByRole("button", { name: "Theo thiết bị" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    act(() => writeThemePreference("dark"));
    expect(themes.getByRole("button", { name: "Tối" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(themes.getByRole("button", { name: "Theo thiết bị" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
  });

  it("paints the previews from the swatch tokens, Device as two halves", async () => {
    open("/app/settings/appearance");
    const themes = await screen.findByRole("group", { name: "Chủ đề" });
    const halves = (theme: string) =>
      [...themes.querySelector(`[data-swatch='${theme}']`)!.children].map(
        (half) => /bg-swatch-(light|dark)/.exec(half.className)?.[1],
      );
    expect(halves("light")).toEqual(["light"]);
    expect(halves("dark")).toEqual(["dark"]);
    expect(halves("system")).toEqual(["light", "dark"]);
  });

  it("turns larger text in tests on and off, in the key the engine reads", async () => {
    const user = userEvent.setup();
    open("/app/settings/appearance");
    const larger = await screen.findByRole("switch", {
      name: "Chữ lớn hơn khi làm bài",
    });
    expect(larger).not.toBeChecked();
    expect(larger).toHaveAccessibleDescription("Bài đọc và câu hỏi hiện ở cỡ chữ 18px");

    await user.click(larger);
    expect(larger).toBeChecked();
    expect(localStorage.getItem("quizzivy.testText")).toBe("large");
    expect(readLargerTestText()).toBe(true);

    await user.click(larger);
    expect(larger).not.toBeChecked();
    expect(localStorage.getItem("quizzivy.testText")).toBe("default");
    expect(readLargerTestText()).toBe(false);
  });

  it("opens with larger text on when it was chosen before", async () => {
    localStorage.setItem("quizzivy.testText", "large");
    open("/app/settings/appearance");
    expect(
      await screen.findByRole("switch", { name: "Chữ lớn hơn khi làm bài" }),
    ).toBeChecked();
  });
});

describe("what is typed survives", () => {
  it("a visit to the other sections and a resize across 768", async () => {
    const user = userEvent.setup();
    const width = viewport("desktop");
    open();
    const name = await screen.findByLabelText("Họ và tên");
    await user.type(name, " Bình");

    await user.click(switcher().getByRole("button", { name: "Đăng nhập" }));
    const form = await openPasswordForm(user);
    await user.type(form.current, "mat-khau-cu-1");
    await user.type(form.next, "mat-khau-moi-2");

    await user.click(switcher().getByRole("button", { name: "Giao diện" }));
    act(() => width.resize("phone"));
    expect(screen.getByRole("heading", { level: 1 })).toHaveClass("sr-only");
    expect(screen.getByRole("link", { name: "Quay lại" })).toBeInTheDocument();

    await user.click(switcher().getByRole("button", { name: "Hồ sơ" }));
    expect(screen.getByLabelText("Họ và tên")).toBe(name);
    expect(name).toHaveValue("Nguyễn Văn An Bình");
    expect(screen.getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();

    await user.click(switcher().getByRole("button", { name: "Đăng nhập" }));
    expect(screen.getByLabelText("Mật khẩu mới")).toBe(form.next);
    expect(form.current).toHaveValue("mat-khau-cu-1");
    expect(form.next).toHaveValue("mat-khau-moi-2");
    expect(meter().filled).toBe(4);

    act(() => width.resize("desktop"));
    expect(screen.getByRole("heading", { level: 1 })).not.toHaveClass("sr-only");
    expect(form.next).toHaveValue("mat-khau-moi-2");
    await user.click(switcher().getByRole("button", { name: "Hồ sơ" }));
    expect(name).toHaveValue("Nguyễn Văn An Bình");
  });
});
