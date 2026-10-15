import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import SetPasswordPage from "@/features/auth/pages/SetPasswordPage";
import type { SetPasswordLinkInfo } from "@/features/auth/api";
import { BASE_URL } from "@/lib/api/client";
import { setLocale } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { assertContractRequest, contractJson } from "@tests/support/contractResponse";
import { studentUser, teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

const CARRIED = "carried-in-the-fragment_0123456789-abcdefg";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";
const ROUTER_STATE = { usr: null, key: "landing", idx: 0 };

const LINK: SetPasswordLinkInfo = {
  email: "giaovien@example.com",
  fullName: "Trần Thị Bình",
  purpose: "invite",
  expiresAt: "2026-10-15T09:00:00Z",
};

const TITLE = "Đặt mật khẩu cho giaovien@example.com";
const INVALID =
  "Liên kết này đã hết hạn hoặc đã được dùng. Hãy nhờ trung tâm gửi liên kết mới.";
const SUBMIT = "Đặt mật khẩu và đăng nhập";

interface Seen {
  url: string;
  hashAtSend: string;
  headers: [string, string][];
  body: unknown;
}

let seen: Seen[] = [];

async function record(request: Request) {
  const body: unknown = await request.clone().json();
  seen.push({
    url: request.url,
    hashAtSend: window.location.hash,
    headers: [...request.headers.entries()],
    body,
  });
  return body;
}

function refusal(code: string, message: string) {
  return { error: { code, message, requestId: REQUEST_ID } };
}

function checkAnswers(...answers: (() => Response | Promise<Response>)[]) {
  let call = 0;
  server.use(
    http.post(`${BASE_URL}/auth/set-password/check`, async ({ request }) => {
      assertContractRequest("/auth/set-password/check", "post", await record(request));
      const answer = answers[Math.min(call, answers.length - 1)]!;
      call += 1;
      return answer();
    }),
  );
}

const validLink = () => contractJson("/auth/set-password/check", "post", 200, LINK);

function setAnswers(...answers: (() => Response | Promise<Response>)[]) {
  let call = 0;
  server.use(
    http.post(`${BASE_URL}/auth/set-password`, async ({ request }) => {
      assertContractRequest("/auth/set-password", "post", await record(request));
      const answer = answers[Math.min(call, answers.length - 1)]!;
      call += 1;
      return answer();
    }),
  );
}

const signedIn =
  (user = teacherUser) =>
  () =>
    contractJson("/auth/set-password", "post", 200, {
      accessToken: "fresh-access-token",
      expiresIn: 900,
      user,
    });

function land(hash = `#token=${CARRIED}`) {
  window.history.replaceState(ROUTER_STATE, "", `/set-password${hash}`);
  const router = createMemoryRouter(
    [
      { path: "/set-password", element: <SetPasswordPage /> },
      { path: "/teacher", element: <p>teacher home</p> },
      { path: "/app", element: <p>student home</p> },
      { path: "/change-password", element: <p>change password</p> },
      { path: "/login", element: <p>sign in page</p> },
    ],
    { initialEntries: ["/set-password"] },
  );
  render(
    <StrictMode>
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <RouterProvider router={router} />
      </QueryClientProvider>
    </StrictMode>,
  );
  return router;
}

const newField = () => screen.getByLabelText("Mật khẩu mới");
const againField = () => screen.getByLabelText("Nhập lại mật khẩu");
const submit = () => screen.getByRole("button", { name: SUBMIT });
const checks = () => seen.filter((s) => s.url.endsWith("/auth/set-password/check"));
const sets = () => seen.filter((s) => s.url.endsWith("/auth/set-password"));

async function fillIn(password = "mật-khẩu-mới-9", again = password) {
  const user = userEvent.setup();
  await screen.findByRole("heading", { level: 1, name: TITLE });
  await user.type(newField(), password);
  if (again !== "") await user.type(againField(), again);
  return user;
}

beforeEach(() => {
  seen = [];
  useAuthStore.getState().clearSession();
});

afterEach(async () => {
  cleanup();
  useAuthStore.getState().clearSession();
  window.history.replaceState(null, "", "/");
  await setLocale("vi");
});

describe("the set-password link", () => {
  it("takes the token out of the address bar before its first request, and keeps the router's history state", async () => {
    checkAnswers(validLink);
    const fetches: { path: string; hash: string }[] = [];
    const realFetch = globalThis.fetch;
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation((...args: Parameters<typeof fetch>) => {
        const target = new Request(...args).url;
        fetches.push({ path: new URL(target).pathname, hash: window.location.hash });
        return realFetch(...args);
      });
    try {
      land();
      expect(window.location.hash).toBe("");
      expect(window.location.pathname).toBe("/set-password");
      expect(window.history.state).toEqual(ROUTER_STATE);

      await screen.findByRole("heading", { level: 1, name: TITLE });
      expect(fetches.length).toBeGreaterThan(0);
      expect(fetches.filter((f) => f.hash !== "")).toEqual([]);
      expect(
        fetches.filter((f) => f.path.endsWith("/auth/set-password/check")),
      ).toHaveLength(1);
      expect(checks()).toHaveLength(1);
      expect(checks()[0]!.hashAtSend).toBe("");
      expect(checks()[0]!.body).toEqual({ token: CARRIED });
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("checks the link exactly once, even when React runs its effects twice", async () => {
    checkAnswers(validLink);
    land();
    await screen.findByRole("heading", { level: 1, name: TITLE });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(checks()).toHaveLength(1);
  });

  it("shows a skeleton and says it is checking while the answer is on its way", async () => {
    let release: () => void = () => undefined;
    checkAnswers(
      () =>
        new Promise<Response>((resolve) => {
          release = () => resolve(validLink());
        }),
    );
    land();
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Đang kiểm tra liên kết…",
    );
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    release();
    expect(
      await screen.findByRole("heading", { level: 1, name: TITLE }),
    ).toBeInTheDocument();
  });

  it("names the account, greets it by name and focuses the heading once the link checks out", async () => {
    checkAnswers(validLink);
    land();
    const heading = await screen.findByRole("heading", { level: 1, name: TITLE });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(screen.getByText(/Chào Trần Thị Bình\./)).toBeInTheDocument();
    expect(newField()).toHaveAttribute("autocomplete", "new-password");
    expect(againField()).toHaveAttribute("autocomplete", "new-password");
    expect(screen.queryByText("Khác mật khẩu tạm")).toBeNull();
    expect(
      screen.getAllByRole("button", { name: "Hiện mật khẩu" }).length,
    ).toBeGreaterThanOrEqual(2);
  });

  it("speaks of a new password and the other devices for a reset link", async () => {
    checkAnswers(() =>
      contractJson("/auth/set-password/check", "post", 200, {
        ...LINK,
        purpose: "reset",
      }),
    );
    land();
    expect(
      await screen.findByText(/các thiết bị khác sẽ bị đăng xuất/),
    ).toBeInTheDocument();
  });

  it.each([
    ["no fragment", ""],
    ["an empty token", "#token="],
    ["a fragment without a token", "#hello"],
    ["a token longer than the contract allows", `#token=${"a".repeat(129)}`],
  ])("calls %s an invalid link and sends nothing", async (_, hash) => {
    land(hash);
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Liên kết không còn dùng được",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText(INVALID)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Đến trang đăng nhập" })).toHaveAttribute(
      "href",
      "/login",
    );
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(seen).toEqual([]);
    expect(window.location.hash).toBe("");
  });

  it("says a used, expired or revoked link is no longer usable, and focuses the news", async () => {
    checkAnswers(() =>
      contractJson(
        "/auth/set-password/check",
        "post",
        410,
        refusal("SET_PASSWORD_LINK_INVALID", "Liên kết không còn hiệu lực."),
      ),
    );
    land();
    const heading = await screen.findByRole("heading", {
      level: 1,
      name: "Liên kết không còn dùng được",
    });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(screen.getByText(INVALID)).toBeInTheDocument();
    expect(screen.queryByLabelText("Mật khẩu mới")).toBeNull();
  });

  it("says when the check could not be made, and checks again on Retry", async () => {
    checkAnswers(
      () =>
        contractJson(
          "/auth/set-password/check",
          "post",
          429,
          refusal("RATE_LIMITED", "Bạn thử quá nhiều lần. Hãy chờ một phút."),
        ),
      validLink,
    );
    const user = userEvent.setup();
    land();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Chưa kiểm tra được liên kết",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bạn thử quá nhiều lần. Hãy chờ một phút.",
    );

    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(
      await screen.findByRole("heading", { level: 1, name: TITLE }),
    ).toBeInTheDocument();
    expect(checks()).toHaveLength(2);
    expect(checks()[1]!.body).toEqual({ token: CARRIED });
  });

  it("says the server cannot be reached when the check gets no answer", async () => {
    checkAnswers(() => HttpResponse.error());
    land();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không kết nối được với máy chủ. Vui lòng thử lại.",
    );
  });
});

describe("setting the password", () => {
  it("states the rules, says when the two passwords differ and focuses the field to fix", async () => {
    checkAnswers(validLink);
    land();
    const user = await fillIn("matkhau", "");
    expect(submit()).toHaveAttribute("aria-disabled", "true");
    expect(submit()).toBeEnabled();

    await user.click(submit());
    const rules = within(screen.getByRole("list"));
    expect(rules.getByText("Có số hoặc ký hiệu").closest("li")).toHaveAttribute(
      "data-state",
      "failed",
    );
    expect(newField()).toHaveAttribute("aria-invalid", "true");
    expect(newField()).toHaveFocus();

    await user.type(newField(), "1");
    await user.click(submit());
    expect(screen.getByText("Hai mật khẩu không giống nhau.")).toBeVisible();
    expect(againField()).toHaveAttribute("aria-invalid", "true");
    expect(againField()).toHaveAccessibleDescription("Hai mật khẩu không giống nhau.");
    expect(againField()).toHaveFocus();
    expect(sets()).toEqual([]);
  });

  it("refuses a password longer than the contract allows, without a request", async () => {
    checkAnswers(validLink);
    land();
    const long = `${"a".repeat(512)}1`;
    const user = userEvent.setup();
    await screen.findByRole("heading", { level: 1, name: TITLE });
    await user.click(newField());
    await user.paste(long);
    await user.click(againField());
    await user.paste(long);
    expect(screen.getByText("Mật khẩu chỉ được dài tối đa 512 ký tự.")).toBeVisible();
    await user.click(submit());
    expect(newField()).toHaveFocus();
    expect(sets()).toEqual([]);
  });

  it("signs the account in through the session store and goes to its home, not to change-password", async () => {
    checkAnswers(validLink);
    setAnswers(signedIn(teacherUser));
    const router = land();
    const user = await fillIn();
    await user.click(submit());

    expect(await screen.findByText("teacher home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/teacher");
    expect(sets()).toHaveLength(1);
    expect(sets()[0]!.body).toEqual({ token: CARRIED, newPassword: "mật-khẩu-mới-9" });
    expect(useAuthStore.getState().accessToken).toBe("fresh-access-token");
    expect(useAuthStore.getState().user?.id).toBe(teacherUser.id);
  });

  it("sends a student to the student app", async () => {
    checkAnswers(validLink);
    setAnswers(signedIn(studentUser));
    const router = land();
    const user = await fillIn();
    await user.click(submit());
    expect(await screen.findByText("student home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/app");
  });

  it("works from the keyboard alone: the fields, their show buttons, then Enter", async () => {
    checkAnswers(validLink);
    setAnswers(signedIn(teacherUser));
    land();
    const user = userEvent.setup();
    const heading = await screen.findByRole("heading", { level: 1, name: TITLE });
    await waitFor(() => expect(heading).toHaveFocus());

    await user.tab();
    expect(newField()).toHaveFocus();
    await user.keyboard("mật-khẩu-mới-9");
    await user.tab();
    expect(screen.getAllByRole("button", { name: "Hiện mật khẩu" })[0]).toHaveFocus();
    await user.tab();
    expect(againField()).toHaveFocus();
    await user.keyboard("mật-khẩu-mới-9");
    await user.tab();
    await user.tab();
    expect(submit()).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(await screen.findByText("teacher home")).toBeInTheDocument();
  });

  it("marks the button busy while saving rather than disabling it, and sends once", async () => {
    let release: () => void = () => undefined;
    checkAnswers(validLink);
    setAnswers(
      () =>
        new Promise<Response>((resolve) => {
          release = () => resolve(signedIn(teacherUser)());
        }),
    );
    land();
    const user = await fillIn();
    await user.click(submit());
    const busy = await screen.findByRole("button", { name: "Đang lưu…" });
    expect(busy).toHaveAttribute("aria-busy", "true");
    expect(busy).toBeEnabled();
    await user.click(busy);
    release();
    expect(await screen.findByText("teacher home")).toBeInTheDocument();
    expect(sets()).toHaveLength(1);
  });

  it("shows the passwords-off state when the centre has password sign-in off", async () => {
    checkAnswers(validLink);
    setAnswers(() =>
      contractJson(
        "/auth/set-password",
        "post",
        422,
        refusal(
          "PASSWORD_SIGN_IN_DISABLED",
          "Trung tâm đã tắt đăng nhập bằng mật khẩu.",
        ),
      ),
    );
    land();
    const user = await fillIn();
    await user.click(submit());

    const heading = await screen.findByRole("heading", {
      level: 1,
      name: "Hãy đăng nhập bằng Google",
    });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(
      screen.getByText(/Hãy đăng nhập bằng Google với địa chỉ email/),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Đến trang đăng nhập" })).toHaveAttribute(
      "href",
      "/login",
    );
    expect(useAuthStore.getState().accessToken).toBeNull();
  });

  it("shows the invalid state when the link was used in the meantime", async () => {
    checkAnswers(validLink);
    setAnswers(() =>
      contractJson(
        "/auth/set-password",
        "post",
        410,
        refusal("SET_PASSWORD_LINK_INVALID", "Liên kết không còn hiệu lực."),
      ),
    );
    land();
    const user = await fillIn();
    await user.click(submit());
    const heading = await screen.findByRole("heading", {
      level: 1,
      name: "Liên kết không còn dùng được",
    });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(screen.getByText(INVALID)).toBeVisible();
  });

  it("keeps what was typed after a failure, and sends again on the next press", async () => {
    checkAnswers(validLink);
    setAnswers(
      () =>
        contractJson(
          "/auth/set-password",
          "post",
          400,
          refusal("VALIDATION_FAILED", "Mật khẩu mới không hợp lệ."),
        ),
      () => HttpResponse.error(),
      signedIn(teacherUser),
    );
    land();
    const user = await fillIn();
    await user.click(submit());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Mật khẩu mới không hợp lệ.",
    );
    expect(newField()).toHaveValue("mật-khẩu-mới-9");
    expect(againField()).toHaveValue("mật-khẩu-mới-9");

    await user.click(submit());
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Không kết nối được với máy chủ. Vui lòng thử lại.",
      ),
    );
    expect(newField()).toHaveValue("mật-khẩu-mới-9");

    await user.click(submit());
    expect(await screen.findByText("teacher home")).toBeInTheDocument();
    expect(sets()).toHaveLength(3);
  });

  it("never puts the token in a URL, a header or the page", async () => {
    checkAnswers(validLink);
    setAnswers(signedIn(teacherUser));
    land();
    await screen.findByRole("heading", { level: 1, name: TITLE });
    expect(document.body.innerHTML).not.toContain(CARRIED);
    expect(document.title).not.toContain(CARRIED);
    const user = await fillIn();
    await user.click(submit());
    await screen.findByText("teacher home");

    expect(seen.length).toBeGreaterThanOrEqual(2);
    for (const request of seen) {
      expect(request.url).not.toContain(CARRIED);
      expect(JSON.stringify(request.headers)).not.toContain(CARRIED);
    }
    expect(window.location.href).not.toContain(CARRIED);
  });

  it("speaks English when the reader chose it", async () => {
    await setLocale("en");
    checkAnswers(validLink);
    land();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Set a password for giaovien@example.com",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Set password and sign in" }),
    ).toBeVisible();
  });
});
