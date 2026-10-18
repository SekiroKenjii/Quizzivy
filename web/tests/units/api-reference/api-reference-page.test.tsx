import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { Toaster } from "@/components/ui/sonner";
import ApiReferencePage from "@/features/api-reference/pages/admin/ApiReferencePage";
import { BASE_URL } from "@/lib/api/client";
import { saveDownloadedFile } from "@/lib/saveFile";
import { useAuthStore } from "@/stores/auth";
import { adminUser } from "@tests/support/fixtures";
import { contractJson } from "@tests/support/contractResponse";
import { queryWrapper } from "@tests/support/queryWrapper";
import { server } from "@tests/support/server";
import "@/lib/i18n";

vi.mock("@/lib/saveFile", () => ({ saveDownloadedFile: vi.fn() }));

const SUMMARY = {
  openapi: "3.1.0",
  version: "0.11.7",
  endpointCount: 312,
  groups: [
    { tag: "auth", count: 12 },
    { tag: "teacher-tests", count: 1240 },
    { tag: "added-later", count: 1 },
  ],
};

function forbidden(path: "/admin/api-reference" | "/admin/api-reference/openapi.json") {
  return contractJson(path, "get", 403, {
    error: {
      code: "FORBIDDEN",
      message: "Bạn không có quyền xem tài liệu API.",
      requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
    },
  });
}

function readsSummary(body: typeof SUMMARY = SUMMARY) {
  server.use(
    http.get(`${BASE_URL}/admin/api-reference`, () =>
      contractJson("/admin/api-reference", "get", 200, body),
    ),
  );
}

function fakeTab() {
  return { opener: {} as unknown, location: { href: "about:blank" }, close: vi.fn() };
}

function renderPage() {
  return render(
    <>
      <ApiReferencePage />
      <Toaster />
    </>,
    { wrapper: queryWrapper() },
  );
}

beforeEach(() => {
  useAuthStore.getState().setSession("token-a", adminUser);
  vi.mocked(saveDownloadedFile).mockReset();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
  vi.restoreAllMocks();
});

describe("the API reference screen", () => {
  it("heads the page and says what the contract holds once it is read", async () => {
    readsSummary();
    renderPage();

    expect(
      screen.getByRole("heading", { level: 1, name: "Tài liệu API" }),
    ).toBeVisible();
    expect(
      await screen.findByText("OpenAPI 3.1.0 · v0.11.7 · 312 endpoint"),
    ).toBeVisible();
    expect(screen.getByText(BASE_URL)).toBeVisible();
  });

  it("draws a group as a card with its words, its count and its path", async () => {
    readsSummary();
    renderPage();

    const card = await screen.findByRole("button", { name: /Đăng nhập & tài khoản/ });
    expect(within(card).getByText("12 endpoint")).toBeVisible();
    expect(within(card).getByText("/auth/*")).toBeVisible();
    expect(within(card).getByText(/Đăng nhập, Google, phiên làm mới/)).toBeVisible();
  });

  it("groups the thousands of a count by the viewer's language", async () => {
    readsSummary();
    renderPage();

    const card = await screen.findByRole("button", { name: /Bài kiểm tra/ });
    expect(within(card).getByText("1.240 endpoint")).toBeVisible();
  });

  it("names a tag it has no words for by the tag itself", async () => {
    readsSummary();
    renderPage();

    const card = await screen.findByRole("button", { name: /added-later/ });
    expect(within(card).getByText("1 endpoint")).toBeVisible();
  });

  it("says so, and offers another go, when the summary cannot be read", async () => {
    server.use(
      http.get(`${BASE_URL}/admin/api-reference`, () =>
        forbidden("/admin/api-reference"),
      ),
    );
    renderPage();

    expect(
      await screen.findByText("Không tải được thông tin tài liệu API."),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Mở tài liệu" })).toBeVisible();
  });
});

describe("opening the reference", () => {
  beforeEach(() => readsSummary());

  it("opens the tab in the click itself, cuts it off from this page, then points it at the docs", async () => {
    const tab = fakeTab();
    const open = vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    let finish: () => void = () => undefined;
    server.use(
      http.post(
        `${BASE_URL}/admin/docs-session`,
        () =>
          new Promise<Response>((resolve) => {
            finish = () => resolve(new Response(null, { status: 204 }));
          }),
      ),
    );
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Mở tài liệu" }));
    expect(open).toHaveBeenCalledWith("about:blank", "_blank");
    expect(tab.opener).toBeNull();
    expect(tab.location.href).toBe("about:blank");
    expect(await screen.findByRole("button", { name: "Đang mở…" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );

    finish();
    await waitFor(() => expect(tab.location.href).toBe(`${BASE_URL}/docs`));
    expect(
      await screen.findByRole("button", { name: "Mở tài liệu" }),
    ).not.toHaveAttribute("aria-disabled");
  });

  it("opens the reference at the group whose card was pressed", async () => {
    const tab = fakeTab();
    vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    server.use(
      http.post(
        `${BASE_URL}/admin/docs-session`,
        () => new Response(null, { status: 204 }),
      ),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole("button", { name: /Bài kiểm tra/ }));

    await waitFor(() =>
      expect(tab.location.href).toBe(`${BASE_URL}/docs#tag/teacher-tests`),
    );
  });

  it("starts one session however often the button is pressed while it is out", async () => {
    const tab = fakeTab();
    const open = vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    let sessions = 0;
    let finish: () => void = () => undefined;
    server.use(
      http.post(`${BASE_URL}/admin/docs-session`, () => {
        sessions += 1;
        return new Promise<Response>((resolve) => {
          finish = () => resolve(new Response(null, { status: 204 }));
        });
      }),
    );
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Mở tài liệu" }));
    fireEvent.click(await screen.findByRole("button", { name: "Đang mở…" }));
    fireEvent.click(screen.getByRole("button", { name: "Đang mở…" }));

    expect(open).toHaveBeenCalledOnce();
    finish();
    await waitFor(() => expect(tab.location.href).toBe(`${BASE_URL}/docs`));
    expect(sessions).toBe(1);
  });

  it("closes the tab and says so when the session cannot be made, and tries again on request", async () => {
    const tab = fakeTab();
    vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    let answer = 500;
    server.use(
      http.post(
        `${BASE_URL}/admin/docs-session`,
        () => new Response(null, { status: answer }),
      ),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: "Mở tài liệu" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không mở được tài liệu API.",
    );
    expect(tab.close).toHaveBeenCalledOnce();

    answer = 204;
    const second = fakeTab();
    vi.spyOn(window, "open").mockReturnValue(second as unknown as Window);
    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    await waitFor(() => expect(second.location.href).toBe(`${BASE_URL}/docs`));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("tries again at the group a failed card asked for", async () => {
    vi.spyOn(window, "open").mockReturnValue(fakeTab() as unknown as Window);
    let answer = 500;
    server.use(
      http.post(
        `${BASE_URL}/admin/docs-session`,
        () => new Response(null, { status: answer }),
      ),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByRole("button", { name: /Bài kiểm tra/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không mở được tài liệu API.",
    );

    answer = 204;
    const second = fakeTab();
    vi.spyOn(window, "open").mockReturnValue(second as unknown as Window);
    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    await waitFor(() =>
      expect(second.location.href).toBe(`${BASE_URL}/docs#tag/teacher-tests`),
    );
  });

  it("asks for pop-ups instead of making a session when the browser blocks the tab", async () => {
    vi.spyOn(window, "open").mockReturnValue(null);
    let sessions = 0;
    server.use(
      http.post(`${BASE_URL}/admin/docs-session`, () => {
        sessions += 1;
        return new Response(null, { status: 204 });
      }),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: "Mở tài liệu" }));

    expect(screen.getByRole("alert")).toHaveTextContent("Trình duyệt đã chặn tab mới.");
    expect(sessions).toBe(0);
  });

  it("closes the tab and does not point it anywhere when the person signed out meanwhile", async () => {
    const tab = fakeTab();
    vi.spyOn(window, "open").mockReturnValue(tab as unknown as Window);
    let finish: () => void = () => undefined;
    server.use(
      http.post(
        `${BASE_URL}/admin/docs-session`,
        () =>
          new Promise<Response>((resolve) => {
            finish = () => resolve(new Response(null, { status: 204 }));
          }),
      ),
    );
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "Mở tài liệu" }));
    await screen.findByRole("button", { name: "Đang mở…" });
    useAuthStore.getState().clearSession();
    finish();

    await waitFor(() => expect(tab.close).toHaveBeenCalledOnce());
    expect(tab.location.href).toBe("about:blank");
  });
});

describe("the contract file and the base URL", () => {
  beforeEach(() => readsSummary());

  it("saves openapi.json under the name the server gives it", async () => {
    server.use(
      http.get(
        `${BASE_URL}/admin/api-reference/openapi.json`,
        () =>
          new Response('{"openapi":"3.1.0"}', {
            headers: {
              "Content-Type": "application/json",
              "Content-Disposition": 'attachment; filename="quizzivy-openapi.json"',
            },
          }),
      ),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: "openapi.json" }));

    await waitFor(() => expect(saveDownloadedFile).toHaveBeenCalledOnce());
    const [file, fallback] = vi.mocked(saveDownloadedFile).mock.calls[0]!;
    expect(file.filename).toBe("quizzivy-openapi.json");
    expect(fallback).toBe("quizzivy-openapi.json");
  });

  it("says so when the file cannot be downloaded", async () => {
    server.use(
      http.get(`${BASE_URL}/admin/api-reference/openapi.json`, () =>
        forbidden("/admin/api-reference/openapi.json"),
      ),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: "openapi.json" }));

    expect(
      await screen.findByText("Bạn không có quyền xem tài liệu API."),
    ).toBeVisible();
    expect(saveDownloadedFile).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "openapi.json" })).not.toHaveAttribute(
        "aria-disabled",
      ),
    );
  });

  it("copies the base URL", async () => {
    const user = userEvent.setup();
    const write = vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
    renderPage();

    await user.click(screen.getByRole("button", { name: /Sao chép/ }));

    expect(write).toHaveBeenCalledWith(BASE_URL);
  });
});
