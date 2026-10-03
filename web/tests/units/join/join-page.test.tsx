import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { delay, http } from "msw";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import JoinPage from "@/features/join/pages/JoinPage";
import { readJoinContext, saveJoinContext } from "@/features/join/context";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  adminUser,
  adminWhoTakesTests,
  sampleClass,
  studentUser,
} from "@tests/support/fixtures";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const CODE = "K7QM2PXA";
const OTHER = "W3RT8KDZ";
const CLASS_NAME = "TOEIC 600 Weekend";
const TEACHER = "Hoàng Thương";
const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

let previews: string[] = [];

function previewFinds() {
  server.use(
    http.post(`${BASE}/join/preview`, async ({ request }) => {
      const { joinCode } = (await request.json()) as { joinCode: string };
      previews.push(joinCode);
      return contractJson("/join/preview", "post", 200, {
        classId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
        className: CLASS_NAME,
        teacherName: TEACHER,
      });
    }),
  );
}

function previewFails(status: number, code: string, message: string) {
  server.use(
    http.post(`${BASE}/join/preview`, async ({ request }) => {
      const { joinCode } = (await request.json()) as { joinCode: string };
      previews.push(joinCode);
      return contractJson("/join/preview", "post", status, {
        error: { code, message, requestId: REQUEST_ID },
      });
    }),
  );
}

function renderJoin(entry = "/join") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const routes: RouteObject[] = [
    { path: "/join", element: <JoinPage /> },
    { path: "/join/:code", element: <JoinPage /> },
    { path: "/login", element: <p>login page</p> },
    { path: "/admin", element: <p>admin home</p> },
  ];
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

const field = () => screen.getByLabelText("Mã lớp");

beforeEach(() => {
  previews = [];
  sessionStorage.clear();
  useAuthStore.getState().clearSession();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("/join", () => {
  it("asks for the code in one large field", () => {
    renderJoin();
    expect(
      screen.getByRole("heading", { level: 1, name: "Tham gia lớp học" }),
    ).toBeVisible();
    expect(screen.getByText("Nhập mã 8 ký tự giáo viên gửi cho bạn.")).toBeVisible();
    expect(field()).toHaveAttribute("maxlength", "9");
    expect(field()).toHaveAttribute("autocomplete", "off");
    expect(field()).toHaveAttribute("placeholder", "K7QM-2PXA");
  });

  it("upper-cases, drops spaces and dashes, and groups as it is typed", async () => {
    previewFinds();
    const user = userEvent.setup();
    renderJoin();
    await user.type(field(), "k7qm");
    expect(field()).toHaveValue("K7QM");
    await user.type(field(), " -2p");
    expect(field()).toHaveValue("K7QM-2P");
    await user.type(field(), "xaZZ");
    expect(field()).toHaveValue("K7QM-2PXA");
  });

  it("keeps a pasted code whole, however it is spaced", async () => {
    previewFinds();
    const user = userEvent.setup();
    renderJoin();
    await user.click(field());
    await user.paste("k7qm - 2pxa");
    expect(field()).toHaveValue("K7QM-2PXA");
  });

  it("says a code never uses 0, O, 1 or I, and sends nothing", async () => {
    previewFinds();
    const user = userEvent.setup();
    renderJoin();
    await user.type(field(), "K7Q0-2PXA");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Mã lớp không bao giờ có 0, O, 1 hoặc I.",
    );
    expect(field()).toHaveAttribute("aria-invalid", "true");
    await new Promise((resolve) => setTimeout(resolve, 400));
    expect(previews).toEqual([]);
  });

  it("looks a code up once it is complete, and only once per code", async () => {
    previewFinds();
    const user = userEvent.setup();
    renderJoin();
    await user.type(field(), CODE);
    expect(await screen.findByText(CLASS_NAME)).toBeVisible();
    expect(previews).toEqual([CODE]);

    await user.clear(field());
    await user.type(field(), OTHER);
    await waitFor(() => expect(previews).toEqual([CODE, OTHER]));
    await user.clear(field());
    await user.type(field(), CODE);
    expect(await screen.findByText(CLASS_NAME)).toBeVisible();
    await new Promise((resolve) => setTimeout(resolve, 400));
    expect(previews).toEqual([CODE, OTHER]);
  });

  it("does not ask twice about a code the server refused", async () => {
    previewFails(404, "JOIN_CODE_EXPIRED", "Mã lớp đã hết hạn.");
    const user = userEvent.setup();
    renderJoin();
    await user.type(field(), CODE);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không có lớp nào dùng mã này. Hãy kiểm tra lại với giáo viên.",
    );
    await user.clear(field());
    await user.type(field(), OTHER);
    await waitFor(() => expect(previews).toEqual([CODE, OTHER]));
    await user.clear(field());
    await user.type(field(), CODE);
    expect(await screen.findByRole("alert")).toBeVisible();
    await new Promise((resolve) => setTimeout(resolve, 400));
    expect(previews).toEqual([CODE, OTHER]);
  });

  it("asks again about a code whose lookup could not be made", async () => {
    previewFails(500, "INTERNAL", "Lỗi máy chủ.");
    const user = userEvent.setup();
    renderJoin();
    await user.type(field(), CODE);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Chưa kiểm tra được mã. Vui lòng thử lại.",
    );
    await user.clear(field());
    await user.type(field(), CODE);
    await waitFor(() => expect(previews).toEqual([CODE, CODE]));
  });

  it("waits for the typing to stop before it looks up", async () => {
    previewFinds();
    const user = userEvent.setup();
    renderJoin();
    await user.type(field(), CODE);
    expect(previews).toEqual([]);
    await waitFor(() => expect(previews).toEqual([CODE]));
  });

  it("asks once about a code retyped while its lookup is out", async () => {
    server.use(
      http.post(`${BASE}/join/preview`, async ({ request }) => {
        const { joinCode } = (await request.json()) as { joinCode: string };
        previews.push(joinCode);
        await delay(300);
        return contractJson("/join/preview", "post", 200, {
          classId: REQUEST_ID,
          className: CLASS_NAME,
          teacherName: TEACHER,
        });
      }),
    );
    const user = userEvent.setup();
    renderJoin();
    await user.type(field(), CODE);
    await waitFor(() => expect(previews).toEqual([CODE]));
    await user.type(field(), "{Backspace}");
    await user.type(field(), "A");
    await screen.findByText(CLASS_NAME);
    await new Promise((resolve) => setTimeout(resolve, 400));
    expect(previews).toEqual([CODE]);
  });

  it("shows the class and its teacher, and nothing more", async () => {
    previewFinds();
    renderJoin(`/join/${CODE}`);
    expect(await screen.findByText(CLASS_NAME)).toBeVisible();
    expect(field()).toHaveValue("K7QM-2PXA");
    expect(field()).toHaveClass("h-[58px]");
    expect(screen.getByText("Đã tìm thấy lớp")).toBeVisible();
    expect(screen.getByText("Đã tìm thấy lớp")).not.toHaveClass("sr-only");
    expect(screen.getByText(TEACHER)).toBeVisible();
    expect(screen.queryByText(/học viên/)).toBeNull();
    expect(
      screen.getByText("Tiếp theo, bạn sẽ đăng nhập hoặc tạo tài khoản."),
    ).toBeVisible();
  });

  it.each([
    "JOIN_CODE_INVALID",
    "JOIN_CODE_EXPIRED",
    "JOIN_CODE_EXHAUSTED",
    "JOIN_CODE_REVOKED",
  ])("gives %s the one plain message", async (code) => {
    previewFails(404, code, "Mã lớp này đã hết hạn.");
    renderJoin(`/join/${CODE}`);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không có lớp nào dùng mã này. Hãy kiểm tra lại với giáo viên.",
    );
    expect(field()).toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByRole("button", { name: /Tham gia/ })).toBeNull();
  });

  it("shows the server's words when it is asked to slow down", async () => {
    previewFails(429, "RATE_LIMITED", "Bạn thử quá nhiều lần. Hãy chờ một phút.");
    renderJoin(`/join/${CODE}`);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bạn thử quá nhiều lần. Hãy chờ một phút.",
    );
  });

  it("remembers the class and sends a signed-out visitor to sign in", async () => {
    previewFinds();
    const user = userEvent.setup();
    const router = renderJoin(`/join/${CODE}`);
    await user.click(
      await screen.findByRole("button", { name: `Tham gia ${CLASS_NAME}` }),
    );
    expect(router.state.location.pathname).toBe("/login");
    expect(readJoinContext()).toEqual({
      code: CODE,
      className: CLASS_NAME,
      teacherName: TEACHER,
    });
  });

  it("forgets the class when the visitor chooses to sign in instead", async () => {
    saveJoinContext({ code: CODE, className: CLASS_NAME, teacherName: TEACHER });
    const user = userEvent.setup();
    const router = renderJoin();
    await user.click(screen.getByRole("link", { name: "Đăng nhập" }));
    expect(router.state.location.pathname).toBe("/login");
    expect(readJoinContext()).toBeNull();
  });

  it("joins a signed-in student at once", async () => {
    previewFinds();
    let joined: unknown = null;
    server.use(
      http.post(`${BASE}/app/classes/join`, async ({ request }) => {
        joined = await request.json();
        return contractJson("/app/classes/join", "post", 200, {
          ...sampleClass,
          name: CLASS_NAME,
        });
      }),
    );
    useAuthStore.getState().setSession("token", studentUser);
    const user = userEvent.setup();
    renderJoin(`/join/${CODE}`);
    await user.click(
      await screen.findByRole("button", { name: `Tham gia ${CLASS_NAME}` }),
    );
    expect(
      await screen.findByRole("heading", { name: `Bạn đã vào lớp ${CLASS_NAME}` }),
    ).toBeVisible();
    expect(joined).toEqual({ joinCode: CODE });
    expect(screen.queryByText("Đã ở trong một lớp?")).toBeNull();
  });

  it.each([
    ["the Admin", adminUser],
    ["an Admin who takes tests", adminWhoTakesTests],
  ])("tells %s that joining is for students", async (_, account) => {
    previewFinds();
    useAuthStore.getState().setSession("token", account);
    const user = userEvent.setup();
    const router = renderJoin(`/join/${CODE}`);
    expect(
      await screen.findByText("Chỉ tài khoản học viên mới tham gia lớp được."),
    ).toBeVisible();
    expect(screen.queryByRole("button", { name: /Tham gia/ })).toBeNull();
    await user.click(screen.getByRole("link", { name: "Về trang chủ của tôi" }));
    expect(router.state.location.pathname).toBe("/admin");
  });
});
