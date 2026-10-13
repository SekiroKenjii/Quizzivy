import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import ImportWorkspacePage from "@/features/imports/pages/teacher/ImportWorkspacePage";
import type { WordImport } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { teacherUser } from "@tests/support/fixtures";
import { useAuthStore } from "@/stores/auth";
import { aiOn } from "../ai/support";
import {
  BASE,
  IMPORT_ID,
  TEST_ID,
  capabilities,
  run,
  source,
  wordImport,
} from "./fixtures";
import "@/lib/i18n";

let current: WordImport;

beforeEach(() => {
  current = wordImport({ status: "processing", run: run() });
  server.use(
    capabilities(),
    http.get(`${BASE}/teacher/imports/limits`, () =>
      contractJson("/teacher/imports/limits", "get", 200, {
        pasteMaxCharacters: 100000,
        maxBytes: 25 * 1024 * 1024,
        formats: ["docx", "pdf"],
      }),
    ),
    http.get(`${BASE}/teacher/imports/:id`, () =>
      contractJson("/teacher/imports/{id}", "get", 200, current),
    ),
  );
});

function renderAt(entry: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/new", element: <ImportWorkspacePage /> },
      { path: "/teacher/imports/:id", element: <ImportWorkspacePage /> },
      { path: "/teacher/imports/:id/confirm", element: <p>confirm page</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

function paneCell(name: string) {
  return screen.getByRole("region", { name }).parentElement!;
}

describe("the AI reader's privacy note", () => {
  const note =
    "Trang quét hoặc ảnh chụp trong lượt nhập này được một dịch vụ AI bên ngoài Quizzivy đọc.";

  it("tells the Sources pane of an import that asked for AI where scanned pages go", async () => {
    current = wordImport({ status: "processing", run: run(), readWithAi: true });
    renderAt(`/teacher/imports/${IMPORT_ID}`);
    const sources = await screen.findByRole("region", { name: "Nguồn" });
    expect(within(sources).getByText(new RegExp(note))).toBeInTheDocument();
    expect(
      within(sources).getByText(/Trang có chữ chọn được không bị gửi đi/),
    ).toBeInTheDocument();
  });

  it("says it in a file import that is still awaiting its files", async () => {
    current = wordImport({
      status: "awaiting_sources",
      sources: [source("exam", { filename: "scan.pdf", format: "pdf" })],
      readWithAi: true,
    });
    renderAt(`/teacher/imports/${IMPORT_ID}`);
    const sources = await screen.findByRole("region", { name: "Nguồn" });
    expect(await within(sources).findByText(new RegExp(note))).toBeInTheDocument();
  });

  it("names the AI reader on the reading stage of an import that asked for it", async () => {
    current = wordImport({
      status: "processing",
      run: run(),
      readWithAi: true,
      sources: [source("exam", { filename: "scan.pdf", format: "pdf" })],
    });
    renderAt(`/teacher/imports/${IMPORT_ID}`);
    const status = await screen.findByRole("region", { name: "Tiến trình" });
    const reading = within(status).getByText("Đọc nội dung").closest("li")!;
    expect(within(reading).getByText("Bản quét được đọc bằng AI")).toBeInTheDocument();
    expect(within(status).getAllByText("Bản quét được đọc bằng AI")).toHaveLength(1);
  });

  it.each([[undefined], [false]])(
    "says nothing for an import that did not ask (readWithAi %s)",
    async (readWithAi) => {
      current = wordImport({
        status: "processing",
        run: run(),
        ...(readWithAi === undefined ? {} : { readWithAi }),
      });
      renderAt(`/teacher/imports/${IMPORT_ID}`);
      await screen.findByRole("region", { name: "Nguồn" });
      expect(screen.queryByText(new RegExp(note))).toBeNull();
      expect(screen.queryByText("Bản quét được đọc bằng AI")).toBeNull();
    },
  );
});

describe("the draft's Read with AI card (DG-207)", () => {
  beforeEach(() =>
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    ),
  );

  afterEach(() => {
    vi.unstubAllGlobals();
    useAuthStore.setState({ accessToken: null, user: null });
  });

  it("sits in the Sources pane after the files and before the privacy note, which gains the AI sentence while it is on", async () => {
    aiOn();
    renderAt("/teacher/imports/new");
    const sources = await screen.findByRole("region", { name: "Nguồn" });
    const toggle = await within(sources).findByRole("switch", { name: "Đọc bằng AI" });
    const key = within(sources).getByLabelText("Tệp đáp án");
    const privacy = within(sources).getByText(
      "Nội dung của bạn được xử lý ở đâu.",
    ).parentElement!;
    expect(
      key.compareDocumentPosition(toggle) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      toggle.compareDocumentPosition(privacy) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(privacy).not.toHaveTextContent(/dịch vụ AI bên ngoài Quizzivy/);
    toggle.click();
    await waitFor(() =>
      expect(privacy).toHaveTextContent(
        /Trang quét hoặc ảnh chụp trong lượt nhập này được một dịch vụ AI bên ngoài Quizzivy đọc\. Trang có chữ chọn được không bị gửi đi\.$/,
      ),
    );
  });

  it("is not drawn on an import that already exists", async () => {
    aiOn();
    useAuthStore.setState({
      user: { ...teacherUser, preferences: { ai: { readImports: true } } },
    });
    current = wordImport({ status: "awaiting_sources", sources: [] });
    renderAt(`/teacher/imports/${IMPORT_ID}`);
    await screen.findByLabelText("Tệp đề thi");
    expect(screen.queryByRole("switch", { name: "Đọc bằng AI" })).toBeNull();
  });
});

describe("the import workspace (DG-207)", () => {
  it("puts the two panes side by side from 960px of content and the Sources pane first in a draft", async () => {
    renderAt("/teacher/imports/new");
    const sources = await screen.findByRole("region", { name: "Nguồn" });
    const grid = sources.parentElement!;
    expect(grid).toHaveClass("grid", "@[960px]/workspace:grid-cols-(--pane-split)");
    expect(grid.parentElement).toHaveClass("@container/workspace");
    expect(grid.firstElementChild).toBe(sources);
    expect(paneCell("Các bước sẽ chạy")).not.toHaveClass("order-first");
  });

  it("puts the status pane first in the stacked order once processing has started", async () => {
    renderAt(`/teacher/imports/${IMPORT_ID}`);
    await screen.findByRole("region", { name: "Tiến trình" });
    expect(paneCell("Tiến trình")).toHaveClass(
      "order-first",
      "@[960px]/workspace:order-none",
    );
    const sources = screen.getByRole("region", { name: "Nguồn" });
    expect(within(sources).getAllByText("Đang đọc")).toHaveLength(
      current.sources.length,
    );
    expect(screen.getByRole("link", { name: "Lịch sử nhập" })).toHaveAttribute(
      "href",
      "/teacher/imports",
    );
  });

  it("names each file's state on its card while queued, and offers Cancel", async () => {
    current = wordImport({
      status: "queued",
      run: run({ status: "queued", stage: "queued", attempt: 0 }),
      sources: [source("exam"), source("answer_key")],
    });
    renderAt(`/teacher/imports/${IMPORT_ID}`);
    const sources = await screen.findByRole("region", { name: "Nguồn" });
    expect(within(sources).getAllByText("Đang chờ")).toHaveLength(2);
    const status = screen.getByRole("region", { name: "Tiến trình" });
    expect(
      within(status).getByText("Đang ở bước: Đang chờ đến lượt"),
    ).toBeInTheDocument();
    expect(within(status).getByRole("button", { name: "Huỷ xử lý" })).toBeEnabled();
  });

  it("offers a new import, not another run, once an import was cancelled", async () => {
    current = wordImport({ status: "cancelled", run: run({ status: "cancelled" }) });
    renderAt(`/teacher/imports/${IMPORT_ID}`);
    const status = await screen.findByRole("region", { name: "Tiến trình" });
    expect(within(status).getByText("Lần nhập đã được đóng")).toBeInTheDocument();
    expect(within(status).getByRole("link", { name: "Nhập đề mới" })).toHaveAttribute(
      "href",
      "/teacher/imports/new",
    );
    expect(screen.queryByRole("button", { name: /Thử xử lý lại/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Đổi tệp/ })).toBeNull();
  });

  it("shows a committed import on its confirm page", async () => {
    current = wordImport({ status: "committed", testId: TEST_ID });
    const router = renderAt(`/teacher/imports/${IMPORT_ID}`);
    expect(await screen.findByText("confirm page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(
      `/teacher/imports/${IMPORT_ID}/confirm`,
    );
    expect(router.state.historyAction).toBe("REPLACE");
  });
});
