import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import NewImportPage from "@/features/imports/pages/teacher/NewImportPage";
import { CrumbTailContext, type PageCrumb } from "@/layouts/shell/crumbs";
import { dayMonth } from "@/lib/i18n/datetime";
import { notify } from "@/lib/toast";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, IMPORT_ID, errorBody, source, wordImport } from "./fixtures";
import "@/lib/i18n";

vi.mock("@/lib/toast", () => ({
  notify: { success: vi.fn(), info: vi.fn(), warning: vi.fn(), error: vi.fn() },
}));

type Call =
  | { kind: "create"; requestId: string; title: string }
  | { kind: "paste"; uploadId: string; expectedRevision: number; text: string }
  | { kind: "read" }
  | { kind: "process"; requestId: string; expectedRevision: number };

type Reply = { status: 409 | 429; code: string; message: string };

let calls: Call[] = [];
let revision = 1;
let pasteMax = 100000;
let pasteReplies: Reply[] = [];
let processReplies: Reply[] = [];

const READY = [
  "Unit 5 Quick Check",
  "Part 1. Choose the best answer.",
  "1. She ___ in Hanoi since 2019.",
  "A. lives",
  "B. has lived *",
  "2. They ___ here for years.",
  "A. live",
  "B. have lived",
].join("\n");

function imported(over: Parameters<typeof wordImport>[0] = {}) {
  return wordImport({
    status: "awaiting_sources",
    revision,
    sourceRevision: 0,
    sources: [],
    ...over,
  });
}

beforeEach(() => {
  calls = [];
  revision = 1;
  pasteMax = 100000;
  pasteReplies = [];
  processReplies = [];
  vi.mocked(notify.info).mockClear();
  vi.mocked(notify.warning).mockClear();
  server.use(
    http.get(`${BASE}/teacher/imports/limits`, () =>
      contractJson("/teacher/imports/limits", "get", 200, {
        pasteMaxCharacters: pasteMax,
        maxBytes: 25 * 1024 * 1024,
        formats: ["docx", "pdf"],
      }),
    ),
    http.post(`${BASE}/teacher/imports`, async ({ request }) => {
      const body = (await request.json()) as { requestId: string; title: string };
      calls.push({ kind: "create", ...body });
      return contractJson(
        "/teacher/imports",
        "post",
        201,
        imported({ title: body.title }),
      );
    }),
    http.get(`${BASE}/teacher/imports/${IMPORT_ID}`, () => {
      calls.push({ kind: "read" });
      revision += 1;
      return contractJson("/teacher/imports/{id}", "get", 200, imported());
    }),
    http.post(`${BASE}/teacher/imports/:id/sources/text`, async ({ request }) => {
      const body = (await request.json()) as {
        uploadId: string;
        expectedRevision: number;
        text: string;
      };
      calls.push({ kind: "paste", ...body });
      const reply = pasteReplies.shift();
      if (reply !== undefined)
        return contractJson(
          "/teacher/imports/{id}/sources/text",
          "post",
          reply.status,
          errorBody(reply.code, reply.message),
        );
      revision += 1;
      const exam = source("exam", {
        filename: "pasted-text.txt",
        format: "text",
        characters: body.text.length,
      });
      return contractJson("/teacher/imports/{id}/sources/text", "post", 201, {
        import: imported({ sourceRevision: 1, sources: [exam] }),
        source: exam,
        sourceRevision: 1,
      });
    }),
    http.post(`${BASE}/teacher/imports/:id/process`, async ({ request }) => {
      const body = (await request.json()) as {
        requestId: string;
        expectedRevision: number;
      };
      calls.push({ kind: "process", ...body });
      const reply = processReplies.shift();
      if (reply !== undefined)
        return contractJson(
          "/teacher/imports/{id}/process",
          "post",
          reply.status,
          errorBody(reply.code, reply.message),
        );
      return contractJson(
        "/teacher/imports/{id}/process",
        "post",
        202,
        wordImport({ status: "queued", revision: revision + 1 }),
      );
    }),
  );
});

function renderPage(entry = "/teacher/imports/new?source=paste") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const crumbs: (readonly PageCrumb[] | null)[] = [];
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/new", element: <NewImportPage /> },
      { path: "/teacher/imports/:id", element: <p>detail page</p> },
      { path: "/teacher/imports", element: <p>history page</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <CrumbTailContext.Provider value={(tail) => crumbs.push(tail)}>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </CrumbTailContext.Provider>,
  );
  return { ...userEvent.setup(), router, crumbs };
}

function box() {
  return screen.getByRole("textbox", { name: "Nội dung đề" });
}

function start() {
  return screen.getByRole("button", { name: "Bắt đầu xử lý" });
}

function paste(value: string) {
  fireEvent.change(box(), { target: { value } });
}

function clipboard(value: { readText: () => Promise<string> } | undefined) {
  Object.defineProperty(navigator, "clipboard", { value, configurable: true });
}

async function ready() {
  await screen.findByText(/^Tìm thấy \d+ câu hỏi\./);
  await waitFor(() => expect(start()).toBeEnabled());
}

describe("the paste page before anything is pasted", () => {
  it("reads Paste a test, names its crumb, and draws no sample button", async () => {
    const { crumbs } = renderPage();
    expect(
      screen.getByRole("heading", { level: 1, name: "Dán đề" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/^Sao chép đề từ Word, Google Docs/)).toBeInTheDocument();
    expect(crumbs.at(-1)).toEqual([{ label: "Dán đề" }]);
    expect(box()).toHaveClass("font-mono");
    expect(await screen.findByText("0 / 100.000 ký tự")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /mẫu|sample/i })).toBeNull();
    expect(screen.queryByRole("button", { name: "Xoá hết" })).toBeNull();
    expect(screen.queryByText(/Văn bản thuần/)).toBeNull();
    expect(screen.queryByText("Đã tìm thấy")).toBeNull();
  });

  it("keeps the layout tips open while the box is empty and offers the example", () => {
    renderPage();
    const toggle = screen.getByRole("button", { name: "Cách trình bày văn bản dán" });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Đánh số mọi câu hỏi")).toBeVisible();
    expect(screen.getByText(/Answer key: 3-since 4-for/)).toBeVisible();
    expect(screen.getByRole("button", { name: "Sao chép ví dụ" })).toBeVisible();
  });

  it("disables Start processing and asks for the content", () => {
    renderPage();
    expect(start()).toBeDisabled();
    expect(screen.getByText("Dán nội dung đề để tiếp tục.")).toBeInTheDocument();
  });
});

describe("what the page finds in pasted text", () => {
  it("counts, names the kind, fills the title and folds the tips away", async () => {
    renderPage();
    paste(READY);
    expect(await screen.findByText("Đã tìm thấy")).toBeInTheDocument();
    const found = screen.getByText("Đã tìm thấy").parentElement!;
    expect(within(found).getByText("Phần").nextSibling).toHaveTextContent("1");
    expect(within(found).getByText("Câu hỏi").nextSibling).toHaveTextContent("2");
    expect(within(found).getByText("Có đáp án").nextSibling).toHaveTextContent("1");
    expect(within(found).getByText("Cần đáp án").nextSibling).toHaveTextContent("1");
    expect(within(found).getByText("Cần đáp án").parentElement).toHaveClass(
      "bg-warning-soft",
    );
    expect(
      screen.getByText("Câu 2 chưa có đáp án. Bạn có thể nhập khi rà soát."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/^Hình ảnh và âm thanh không được dán/),
    ).toBeInTheDocument();
    expect(screen.getByText(/^Văn bản thuần · đánh dấu đáp án/)).toBeInTheDocument();
    expect(screen.getByLabelText("Tên đề")).toHaveValue("Unit 5 Quick Check");
    expect(
      screen.getByText(
        "Tự điền từ dòng đầu tiên khi dòng đó giống tên đề. Bạn có thể sửa.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Cách trình bày văn bản dán" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("Đánh số mọi câu hỏi")).not.toBeVisible();
    await waitFor(() => expect(start()).toBeEnabled());
    expect(
      screen.getByText(
        "Tìm thấy 2 câu hỏi. Quá trình xử lý mất khoảng một phút. Bạn có thể rời trang này. Nhấn Ctrl + Enter để bắt đầu.",
      ),
    ).toBeInTheDocument();
  });

  it("names six unanswered questions, then how many more", async () => {
    renderPage();
    paste(
      Array.from({ length: 8 }, (_, i) => `${i + 1}. Question ${i + 1}`).join("\n"),
    );
    expect(
      await screen.findByText(
        "Câu 1, 2, 3, 4, 5, 6 và 2 câu khác chưa có đáp án. Bạn có thể nhập khi rà soát.",
      ),
    ).toBeInTheDocument();
  });

  it("names a label used twice in one part, even one the count leaves out", async () => {
    renderPage();
    paste("1. First?\nAnswer: a\n2. Second?\nAnswer: b\n2. Second again?\nAnswer: c");
    expect(
      await screen.findByText(/^Số 2 xuất hiện hai lần trong cùng một phần\./),
    ).toBeInTheDocument();
    const found = screen.getByText("Đã tìm thấy").parentElement!;
    expect(within(found).getByText("Câu hỏi").nextSibling).toHaveTextContent("2");
  });

  it("holds Start back until the server is known to process imports", async () => {
    server.use(
      http.get(
        `${BASE}/teacher/imports/capabilities`,
        () => new Promise<never>(() => {}),
      ),
    );
    renderPage();
    paste(READY);
    await screen.findByText(/^Tìm thấy 2 câu hỏi\./);
    expect(start()).toBeDisabled();
  });

  it("holds Start back when no question is numbered", async () => {
    renderPage();
    paste("Unit 5 Quick Check\nChoose the best answer.");
    expect(
      await screen.findByText(/^Không thấy câu hỏi nào được đánh số\./),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Chưa thấy câu hỏi nào được đánh số. Hãy đánh số mỗi câu như “1.” hoặc “Câu 1”.",
      ),
    ).toBeInTheDocument();
    expect(start()).toBeDisabled();
  });

  it("turns the counter and the box to danger over the server's limit", async () => {
    pasteMax = 40;
    renderPage();
    expect(await screen.findByText("0 / 40 ký tự")).toBeInTheDocument();
    paste(READY);
    const counter = await screen.findByText(`${READY.length} / 40 ký tự`);
    expect(counter).toHaveClass("text-danger-ink");
    expect(box()).toHaveAttribute("aria-invalid", "true");
    expect(box()).toHaveClass("border-danger");
    expect(
      screen.getByText(
        "Văn bản vượt giới hạn 40 ký tự. Hãy chia đề và dán thành hai lần nhập.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Văn bản vượt quá 40 ký tự. Hãy chia thành hai lần nhập."),
    ).toBeInTheDocument();
    expect(start()).toBeDisabled();
  });

  it("counts code points of the normalised text", async () => {
    renderPage();
    paste(`Pha${String.fromCodePoint(0x302, 0x300)}n ${String.fromCodePoint(0x1f600)}`);
    expect(await screen.findByText("6 / 100.000 ký tự")).toBeInTheDocument();
  });
});

describe("the clipboard and clear actions", () => {
  it("replaces the text from the clipboard and moves focus to the box", async () => {
    const user = renderPage();
    clipboard({ readText: () => Promise.resolve(READY) });
    paste("old text");
    await user.click(screen.getByRole("button", { name: "Dán từ bộ nhớ tạm" }));
    await waitFor(() => expect(box()).toHaveValue(READY));
    expect(box()).toHaveFocus();
  });

  it("says so when the clipboard is empty", async () => {
    const user = renderPage();
    clipboard({ readText: () => Promise.resolve("") });
    await user.click(screen.getByRole("button", { name: "Dán từ bộ nhớ tạm" }));
    await waitFor(() =>
      expect(notify.info).toHaveBeenCalledWith("Bộ nhớ tạm đang trống"),
    );
    expect(box()).toHaveValue("");
  });

  it.each([
    ["refuses", { readText: () => Promise.reject(new DOMException("denied")) }],
    ["lacks the clipboard API", undefined],
  ])("tells the reader to press Ctrl+V when the browser %s", async (_name, api) => {
    const user = renderPage();
    clipboard(api);
    await user.click(screen.getByRole("button", { name: "Dán từ bộ nhớ tạm" }));
    await waitFor(() =>
      expect(notify.warning).toHaveBeenCalledWith(
        "Trình duyệt đã chặn bộ nhớ tạm. Hãy bấm vào ô và nhấn Ctrl+V (⌘V trên Mac).",
      ),
    );
  });

  it("clears the box and shows Clear only while there is text", async () => {
    const user = renderPage();
    paste(READY);
    await user.click(screen.getByRole("button", { name: "Xoá hết" }));
    expect(box()).toHaveValue("");
    expect(box()).toHaveFocus();
    expect(screen.queryByRole("button", { name: "Xoá hết" })).toBeNull();
  });
});

describe("starting a pasted import", () => {
  it("creates, sends the normalised text and processes on Ctrl+Enter", async () => {
    renderPage();
    paste(READY.replaceAll("\n", "\r\n"));
    await ready();
    fireEvent.keyDown(box(), { key: "Enter", ctrlKey: true });
    expect(await screen.findByText("detail page")).toBeInTheDocument();
    expect(calls.map((call) => call.kind)).toEqual(["create", "paste", "process"]);
    expect(calls[0]).toMatchObject({ title: "Unit 5 Quick Check" });
    expect(calls[1]).toMatchObject({ expectedRevision: 1, text: READY });
    expect(calls[2]).toMatchObject({ expectedRevision: 2 });
  });

  it("does not start on Enter while an input method is composing", async () => {
    renderPage();
    paste(READY);
    await ready();
    fireEvent.keyDown(box(), { key: "Enter", metaKey: true, isComposing: true });
    fireEvent.keyDown(box(), { key: "Enter" });
    await new Promise((settle) => setTimeout(settle, 50));
    expect(calls).toEqual([]);
  });

  it("names a pasted test without a title line by the date", async () => {
    const user = renderPage();
    paste("1. First?\nAnswer: a");
    await screen.findByText(/^Tìm thấy 1 câu hỏi\./);
    const fallback = `Đề đã dán · ${dayMonth(new Date(), "vi")}`;
    expect(screen.getByLabelText("Tên đề")).toHaveValue("");
    expect(screen.getByLabelText("Tên đề")).toHaveAttribute("placeholder", fallback);
    await user.click(start());
    await screen.findByText("detail page");
    expect(calls[0]).toMatchObject({ kind: "create", title: fallback });
  });

  it("lets a typed title win over the first line", async () => {
    const user = renderPage();
    paste(READY);
    await screen.findByText(/^Tìm thấy 2 câu hỏi\./);
    await user.clear(screen.getByLabelText("Tên đề"));
    await user.type(screen.getByLabelText("Tên đề"), "Kiểm tra Unit 5");
    paste(`${READY}\n3. Third?\nAnswer: c`);
    await user.click(start());
    await screen.findByText("detail page");
    expect(calls[0]).toMatchObject({ title: "Kiểm tra Unit 5" });
  });

  it("retries with the same identities, and an edited text takes a new upload", async () => {
    pasteReplies = [
      { status: 429, code: "IMPORT_BUSY", message: "Hệ thống đang bận." },
      { status: 429, code: "IMPORT_BUSY", message: "Hệ thống đang bận." },
    ];
    const user = renderPage();
    paste(READY);
    await screen.findByText(/^Tìm thấy 2 câu hỏi\./);
    await user.click(start());
    expect(await screen.findByText("Hệ thống đang bận.")).toBeInTheDocument();
    await user.click(start());
    await waitFor(() =>
      expect(calls.filter((c) => c.kind === "paste")).toHaveLength(2),
    );
    const edited = `${READY}\n3. Third?\nAnswer: c`;
    paste(edited);
    await screen.findByText(/^Tìm thấy 3 câu hỏi\./);
    await user.click(start());
    await screen.findByText("detail page");
    const pastes = calls.filter((call) => call.kind === "paste");
    expect(calls.filter((call) => call.kind === "create")).toHaveLength(1);
    expect(pastes).toHaveLength(3);
    expect(pastes[1]!.uploadId).toBe(pastes[0]!.uploadId);
    expect(pastes[2]!.uploadId).not.toBe(pastes[0]!.uploadId);
    expect(pastes[2]!.text).toBe(edited);
  });

  it("does not send the text again when only processing failed", async () => {
    processReplies = [
      { status: 429, code: "IMPORT_BUSY", message: "Hàng đợi đang đầy." },
    ];
    const user = renderPage();
    paste(READY);
    await ready();
    await user.click(start());
    expect(await screen.findByText("Hàng đợi đang đầy.")).toBeInTheDocument();
    await user.click(start());
    await screen.findByText("detail page");
    expect(calls.map((call) => call.kind)).toEqual([
      "create",
      "paste",
      "process",
      "process",
    ]);
    expect(calls[3]).toMatchObject({ expectedRevision: 2 });
  });

  it("re-reads the import after a conflict and sends against the fresh revision", async () => {
    pasteReplies = [
      { status: 409, code: "IMPORT_CONFLICT", message: "Lượt nhập đã thay đổi." },
    ];
    const user = renderPage();
    paste(READY);
    await screen.findByText(/^Tìm thấy 2 câu hỏi\./);
    await user.click(start());
    expect(await screen.findByText("Lượt nhập đã thay đổi.")).toBeInTheDocument();
    expect(calls.map((call) => call.kind)).toEqual(["create", "paste", "read"]);
    await user.click(start());
    await screen.findByText("detail page");
    const pastes = calls.filter((call) => call.kind === "paste");
    expect(pastes[1]!.uploadId).not.toBe(pastes[0]!.uploadId);
    expect(pastes[1]!.expectedRevision).toBe(2);
  });

  it("shows the server's message when intake is busy", async () => {
    pasteReplies = [
      { status: 429, code: "IMPORT_BUSY", message: "Hệ thống đang bận, thử lại sau." },
    ];
    const user = renderPage();
    paste(READY);
    await screen.findByText(/^Tìm thấy 2 câu hỏi\./);
    await user.click(start());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Hệ thống đang bận, thử lại sau.",
    );
    expect(start()).toBeEnabled();
  });
});

describe("leaving with pasted text", () => {
  it("asks before leaving, and Stay keeps the text", async () => {
    const user = renderPage();
    paste(READY);
    void user.router.navigate("/teacher/imports");
    const dialog = await screen.findByRole("dialog", {
      name: "Rời trang mà chưa nhập đề?",
    });
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));
    expect(box()).toHaveValue(READY);
    void user.router.navigate("/teacher/imports");
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Rời trang",
      }),
    );
    expect(await screen.findByText("history page")).toBeInTheDocument();
  });

  it("does not ask when switching the source, nor after starting", async () => {
    const user = renderPage();
    paste(READY);
    await screen.findByText(/^Tìm thấy 2 câu hỏi\./);
    await user.click(screen.getByRole("button", { name: "Tải tệp lên" }));
    await user.click(screen.getByRole("button", { name: "Dán văn bản" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(box()).toHaveValue(READY);
    await user.click(start());
    expect(await screen.findByText("detail page")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not ask with an empty box", async () => {
    const user = renderPage();
    void user.router.navigate("/teacher/imports");
    expect(await screen.findByText("history page")).toBeInTheDocument();
  });
});
