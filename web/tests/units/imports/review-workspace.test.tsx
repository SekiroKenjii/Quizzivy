import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import ImportReviewPage from "@/features/imports/pages/teacher/ImportReviewPage";
import type { SaveImportReview } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import {
  BASE,
  IMPORT_ID,
  capabilities,
  deferred,
  errorBody,
  run,
  source,
  wordImport,
} from "./fixtures";
import {
  baseline,
  card,
  cardHeader,
  isOpen,
  renderReview,
  savedFrom,
  serveReview,
  type ReviewServer,
} from "./reviewHarness";
import "@/lib/i18n";

Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
Range.prototype.getBoundingClientRect ??= () => new DOMRect();
document.elementFromPoint ??= () => null;

let state: ReviewServer;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
  serveReview(baseline(), state);
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function atWidth(initial: number) {
  let width = initial;
  const listeners = new Set<() => void>();
  vi.stubGlobal("matchMedia", (query: string) => {
    const min = /min-width:\s*(\d+)px/.exec(query);
    const max = /max-width:\s*(\d+)px/.exec(query);
    return {
      matches:
        (min === null || width >= Number(min[1])) &&
        (max === null || width <= Number(max[1])),
      media: query,
      onchange: null,
      addEventListener: (_type: string, listener: () => void) =>
        listeners.add(listener),
      removeEventListener: (_type: string, listener: () => void) =>
        listeners.delete(listener),
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    } as unknown as MediaQueryList;
  });
  return (next: number) => {
    width = next;
    act(() => listeners.forEach((listener) => listener()));
  };
}

async function showInSource(user: ReturnType<typeof userEvent.setup>, id: string) {
  if (!isOpen(id)) await user.click(cardHeader(id));
  await user.click(within(card(id)).getByRole("button", { name: "Xem trong nguồn" }));
}

describe("moving through findings", () => {
  it("counts the open findings before one is chosen", async () => {
    await renderReview();
    expect(screen.getByText("3 mục còn mở")).toBeInTheDocument();
  });

  it("continues from a resolved finding instead of going back to the first", async () => {
    server.use(
      http.put(`${BASE}/teacher/imports/:id/review`, async ({ request }) => {
        const body = (await request.json()) as SaveImportReview;
        state.puts.push(body);
        const saved = savedFrom(baseline(), body);
        return contractJson("/teacher/imports/{id}/review", "put", 200, {
          ...saved,
          findings: saved.findings.filter((finding) => finding.id !== "f-missing"),
        });
      }),
    );
    const { user } = await renderReview();
    const next = screen.getByRole("button", { name: "Tiếp" });
    await user.click(next);
    await user.click(next);
    expect(screen.getByText("2 / 3 mục còn mở")).toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: "Lựa chọn A là đáp án đúng" }));
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts).toHaveLength(1));
    expect(await screen.findByText("2 mục còn mở")).toBeInTheDocument();

    await user.click(next);
    await waitFor(() => expect(document.activeElement?.id).toBe("finding-f-irregular"));
  });
});

describe("keyboard focus", () => {
  it("opens the card chosen from its header and closes the one before", async () => {
    const { user } = await renderReview();
    expect(isOpen("q1")).toBe(true);
    await user.click(cardHeader("q2"));
    expect(isOpen("q2")).toBe(true);
    expect(isOpen("q1")).toBe(false);
    expect(cardHeader("q2")).toHaveFocus();
  });

  it("gives the source blocks one tab stop and moves between them with the arrow keys", async () => {
    const { user } = await renderReview();
    const first = await screen.findByRole("button", { name: /to school yesterday/ });
    const second = screen.getByRole("button", { name: /Which colour is the sky/ });
    expect(first).toHaveAttribute("tabindex", "0");
    expect(second).toHaveAttribute("tabindex", "-1");

    first.focus();
    await user.keyboard("{ArrowDown}");
    expect(second).toHaveFocus();
    expect(second).toHaveAttribute("tabindex", "0");
    expect(first).toHaveAttribute("tabindex", "-1");
    await user.keyboard("{Enter}");
    expect(isOpen("q2")).toBe(true);
  });

  it("keeps the review's state when a narrow workspace switches to the source and back", async () => {
    contentWidth(900);
    const { user } = await renderReview();
    const prompt = await screen.findByRole("textbox", { name: "Nội dung câu 1" });
    await user.type(prompt, " nhé");
    await user.click(screen.getByRole("button", { name: "Nguồn" }));
    expect(prompt).not.toBeVisible();
    expect(screen.getByRole("region", { name: "Tài liệu nguồn" })).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Đề" }));
    expect(isOpen("q1")).toBe(true);
    expect(screen.getByRole("textbox", { name: "Nội dung câu 1" })).toHaveTextContent(
      /nhé/,
    );
  });
});

describe("moving between the exam and its source", () => {
  it("moves focus with the view when a narrow workspace switches between them", async () => {
    contentWidth(900);
    const { user } = await renderReview();
    await showInSource(user, "q2");
    const block = screen.getByRole("button", { name: /Which colour is the sky/ });
    await waitFor(() => expect(block).toHaveFocus());

    await user.keyboard("{Enter}");
    await waitFor(() => expect(cardHeader("q2")).toHaveFocus());
    expect(block).not.toBeVisible();
  });

  it("scrolls the source to the chosen question once a narrow workspace shows it", async () => {
    contentWidth(900);
    const scrolled: Element[] = [];
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(function record(
      this: Element,
    ) {
      scrolled.push(this);
    });
    const { user } = await renderReview();
    await user.click(cardHeader("q2"));
    const block = screen.getByRole("button", {
      name: /Which colour is the sky/,
      hidden: true,
    });
    scrolled.length = 0;

    await user.click(screen.getByRole("button", { name: "Nguồn" }));
    await waitFor(() => expect(scrolled).toContain(block));
  });

  it("leaves the exam's scroll position alone on a wide workspace", async () => {
    const { user } = await renderReview();
    const scroller = document.querySelector<HTMLElement>("[data-resize-middle]")!;
    const writes: number[] = [];
    Object.defineProperty(scroller, "scrollTop", {
      configurable: true,
      get: () => 0,
      set: (value: number) => writes.push(value),
    });
    await showInSource(user, "q2");
    await user.click(screen.getByRole("button", { name: /Which colour is the sky/ }));

    expect(isOpen("q2")).toBe(true);
    expect(writes).toEqual([]);
  });

  it("still asks before leaving when a narrowed window hides an unsaved edit", async () => {
    const resize = atWidth(1440);
    server.use(
      http.put(`${BASE}/teacher/imports/:id/review`, async ({ request }) => {
        state.puts.push((await request.json()) as SaveImportReview);
        return new Response(null, { status: 503 });
      }),
    );
    const { user, router } = await renderReview();
    await user.type(screen.getByLabelText("Tên đề"), "A");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts).toHaveLength(1));

    resize(500);
    await act(async () => {
      void router.navigate(`/teacher/imports/${IMPORT_ID}`);
    });
    const dialog = await screen.findByRole("dialog", { name: "Rời trang rà soát?" });
    expect(within(dialog).getByText(/Chưa lưu được thay đổi/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Rời trang" }));
    expect(await screen.findByText("import detail")).toBeInTheDocument();
  });
});

describe("editing a question", () => {
  it("edits the open card's prompt in place and saves it", async () => {
    const { user } = await renderReview();
    const prompt = await within(card("q1")).findByRole("textbox", {
      name: "Nội dung câu 1",
    });
    await user.type(prompt, " nhé");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts).toHaveLength(1));
    const saved = state.puts[0]!.sections[0]!.items[0]!.question!;
    expect(JSON.stringify(saved.prompt)).toContain("nhé");
  });

  it("offers only exclusion in the question's menu", async () => {
    const { user } = await renderReview();
    await user.click(screen.getByRole("button", { name: "Thao tác với câu 1" }));
    const items = await screen.findAllByRole("menuitem");
    expect(items.map((item) => item.textContent)).toEqual(["Loại khỏi đề…"]);
  });

  it.each([
    ["Esc", (user: ReturnType<typeof userEvent.setup>) => user.keyboard("{Escape}")],
    [
      "Cancel",
      (user: ReturnType<typeof userEvent.setup>) =>
        user.click(screen.getByRole("button", { name: "Huỷ" })),
    ],
    [
      "the close button",
      (user: ReturnType<typeof userEvent.setup>) =>
        user.click(screen.getByRole("button", { name: "Đóng" })),
    ],
  ])(
    "returns focus to the card's actions button when the exclusion dialog closes with %s",
    async (_how, close) => {
      const { user } = await renderReview();
      const actions = screen.getByRole("button", { name: "Thao tác với câu 1" });
      await user.click(actions);
      await user.click(await screen.findByRole("menuitem", { name: "Loại khỏi đề…" }));
      await screen.findByRole("dialog", { name: "Loại câu 1?" });

      await close(user);
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      await waitFor(() => expect(actions).toHaveFocus());
    },
  );

  it("returns focus to the card's actions button after excluding from its menu", async () => {
    const { user } = await renderReview();
    const actions = screen.getByRole("button", { name: "Thao tác với câu 1" });
    await user.click(actions);
    await user.click(await screen.findByRole("menuitem", { name: "Loại khỏi đề…" }));
    const dialog = await screen.findByRole("dialog", { name: "Loại câu 1?" });
    await user.type(within(dialog).getByLabelText("Lý do"), "Trùng câu 5");
    await user.click(within(dialog).getByRole("button", { name: "Loại" }));

    await waitFor(() => expect(actions).toHaveFocus());
  });

  it("excludes with a reason, then shows the question read-only with a way back", async () => {
    const { user } = await renderReview();
    await user.click(screen.getByRole("button", { name: "Thao tác với câu 1" }));
    await user.click(await screen.findByRole("menuitem", { name: "Loại khỏi đề…" }));
    const dialog = await screen.findByRole("dialog", { name: "Loại câu 1?" });
    await user.type(within(dialog).getByLabelText("Lý do"), "Trùng câu 5");
    await user.click(within(dialog).getByRole("button", { name: "Loại" }));

    const q1 = card("q1");
    expect(within(q1).getByText("Đã loại khỏi đề: Trùng câu 5")).toBeInTheDocument();
    expect(within(q1).queryByRole("textbox", { name: "Nội dung câu 1" })).toBeNull();
    expect(within(q1).getByRole("group", { name: /Nội dung câu hỏi/ })).toBeVisible();

    await user.click(within(q1).getByRole("button", { name: "Đưa lại vào đề" }));
    expect(within(q1).queryByText(/Đã loại khỏi đề/)).toBeNull();
    expect(
      await within(q1).findByRole("textbox", { name: "Nội dung câu 1" }),
    ).toBeInTheDocument();
  });
});

describe("a pasted import", () => {
  const pasted = source("exam", {
    format: "text",
    characters: 120,
    filename: "stored-original.txt",
  });

  beforeEach(() => {
    server.use(
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport({ sources: [pasted] }),
        ),
      ),
    );
  });

  it("names the pasted text as such and never shows the stored file name", async () => {
    await renderReview();
    const which = screen.getByRole("group", { name: "Chọn tài liệu" });
    expect(
      within(which)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Văn bản đã dán"]);
    const view = screen.getByRole("group", { name: "Cách hiển thị" });
    expect(
      within(view)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual(["Như đã dán"]);
    expect(screen.getByText(/^Văn bản như bạn đã dán/)).toBeInTheDocument();
    expect(screen.queryByText(/stored-original/)).toBeNull();
  });

  it("downloads the pasted source by its id", async () => {
    let requested = "";
    server.use(
      http.get(
        `${BASE}/teacher/imports/:id/sources/:sourceId/download`,
        ({ params }) => {
          requested = String(params.sourceId);
          return contractJson(
            "/teacher/imports/{id}/sources/{sourceId}/download",
            "get",
            410,
            errorBody("IMPORT_FILES_REMOVED", "Bản gốc đã được xoá."),
          );
        },
      ),
    );
    const { user } = await renderReview();
    await user.click(screen.getByRole("button", { name: "Tải bản gốc" }));
    await waitFor(() => expect(requested).toBe(pasted.id));
  });
});

describe("a phone", () => {
  it("shows the review read-only with a note that says where to edit", async () => {
    atWidth(500);
    await renderReview();
    expect(screen.getByRole("note")).toHaveTextContent(/Trên điện thoại/);
    const q1 = card("q1");
    expect(within(q1).queryByRole("textbox", { name: "Nội dung câu 1" })).toBeNull();
    expect(within(q1).getByRole("group", { name: /Nội dung câu hỏi/ })).toBeVisible();
    expect(screen.getByRole("button", { name: "Thao tác với câu 1" })).toBeDisabled();
    expect(screen.getByLabelText("Tên đề")).toBeDisabled();
  });
});

describe("a review that cannot be edited", () => {
  it("still lets the teacher browse a committed review", async () => {
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport({
            status: "committed",
            testId: "018f0000-0000-7000-8000-0000000000e6",
          }),
        ),
      ),
    );
    const { user } = await renderReview();
    expect(screen.getByText(/Phần rà soát giờ chỉ để xem/)).toBeInTheDocument();
    expect(screen.getByLabelText("Tên đề")).toBeDisabled();
    await user.click(cardHeader("q2"));
    const q2 = card("q2");
    expect(
      within(q2).queryByRole("button", { name: "Xác nhận đã kiểm tra" }),
    ).toBeNull();
    expect(within(q2).queryByRole("textbox", { name: "Nội dung câu 2" })).toBeNull();
    expect(within(q2).getByRole("button", { name: "Xem trong nguồn" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Đáp án" }));
    expect(await screen.findByRole("button", { name: /^1\. B/ })).toBeEnabled();
  });

  it("stops saving and refreshes the import when it left review elsewhere", async () => {
    let reads = 0;
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () => {
        reads += 1;
        return contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport(reads > 1 ? { status: "cancelled" } : {}),
        );
      }),
      http.put(`${BASE}/teacher/imports/:id/review`, async ({ request }) => {
        state.puts.push((await request.json()) as SaveImportReview);
        return contractJson(
          "/teacher/imports/{id}/review",
          "put",
          409,
          errorBody("IMPORT_CONFLICT", "Lần nhập không còn ở trạng thái rà soát."),
        );
      }),
    );
    const { user } = await renderReview();
    await user.type(screen.getByLabelText("Tên đề"), "A");
    await vi.advanceTimersByTimeAsync(1500);

    expect(
      await screen.findByText(/Lần nhập đã bị huỷ; phần rà soát chỉ để xem/),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Tên đề")).toBeDisabled();
    await vi.advanceTimersByTimeAsync(5000);
    expect(state.puts).toHaveLength(1);
  });

  it("names a reprocess that did not finish", async () => {
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport({ draftRevision: 2, run: run({ status: "cancelled" }) }),
        ),
      ),
    );
    await renderReview();
    expect(screen.getByText(/Lần xử lý lại gần nhất đã được dừng/)).toBeInTheDocument();
  });

  it("tells a review opened during a reprocess that it finished, and reloads it", async () => {
    let finished = false;
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          finished
            ? wordImport({
                draftRevision: 3,
                run: run({ status: "succeeded", stage: "ready" }),
              })
            : wordImport({ status: "processing", draftRevision: 3, run: run() }),
        ),
      ),
    );
    const { user } = await renderReview();
    expect(screen.getByText(/^Đang xử lý lại\./)).toBeInTheDocument();
    expect(screen.getByLabelText("Tên đề")).toBeDisabled();

    finished = true;
    await vi.advanceTimersByTimeAsync(2500);
    expect(
      await screen.findByText(
        "Việc xử lý đã xong. Tải lại để xem bản rà soát mới nhất.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("Tên đề"),
      "the review on screen predates the result",
    ).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Tải lại" }));
    await waitFor(() => expect(screen.getByLabelText("Tên đề")).toBeEnabled());
    expect(screen.queryByText(/Việc xử lý đã xong/)).toBeNull();
  });

  it("does not claim processing finished when the reprocess failed, and keeps the review editable", async () => {
    let failed = false;
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          failed
            ? wordImport({
                draftRevision: 3,
                run: run({
                  status: "failed",
                  stage: "normalization",
                  errorCode: "CONVERSION_FAILED",
                }),
              })
            : wordImport({ status: "processing", draftRevision: 3, run: run() }),
        ),
      ),
    );
    await renderReview();
    expect(screen.getByLabelText("Tên đề")).toBeDisabled();

    failed = true;
    await vi.advanceTimersByTimeAsync(2500);
    await waitFor(() => expect(screen.getByLabelText("Tên đề")).toBeEnabled());
    expect(screen.queryByText(/Việc xử lý đã xong/)).toBeNull();
  });

  it("links a closed import's review to the import rather than to progress", async () => {
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport({ status: "cancelled", draftRevision: 3 }),
        ),
      ),
    );
    await renderReview();
    expect(screen.getByRole("link", { name: "Xem lần nhập" })).toHaveAttribute(
      "href",
      `/teacher/imports/${IMPORT_ID}`,
    );
    expect(screen.queryByRole("link", { name: "Xem tiến trình" })).toBeNull();
  });
});

describe("the review's data", () => {
  it("never mounts from a review cached before the page opened", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(["word-import-review", IMPORT_ID], {
      ...baseline(),
      draft: { ...baseline().draft, title: "Bản cũ trước khi xử lý lại" },
    });
    const router = createMemoryRouter(
      [{ path: "/teacher/imports/:id/review", element: <ImportReviewPage /> }],
      { initialEntries: [`/teacher/imports/${IMPORT_ID}/review`] },
    );
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    expect(await screen.findByLabelText("Tên đề")).toHaveValue("Đề thi học kỳ 1");
    expect(screen.queryByDisplayValue("Bản cũ trước khi xử lý lại")).toBeNull();
  });

  it("offers a retry inside the summary when the last save failed", async () => {
    let fail = true;
    server.use(
      http.put(`${BASE}/teacher/imports/:id/review`, async ({ request }) => {
        const body = (await request.json()) as SaveImportReview;
        state.puts.push(body);
        if (fail) {
          fail = false;
          return contractJson(
            "/teacher/imports/{id}/review",
            "put",
            422,
            errorBody("VALIDATION_FAILED", "Bản rà soát gửi lên không hợp lệ."),
          );
        }
        return contractJson(
          "/teacher/imports/{id}/review",
          "put",
          200,
          savedFrom(baseline(), body),
        );
      }),
    );
    const { user } = await renderReview();
    await user.type(screen.getByLabelText("Tên đề"), "A");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() =>
      expect(screen.getByText("Chưa lưu được thay đổi gần nhất")).toBeInTheDocument(),
    );

    await user.click(
      screen.getAllByRole("button", { name: "Xem trước và hoàn tất" })[0]!,
    );
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/Chưa lưu được thay đổi/)).toBeInTheDocument();
    expect(within(dialog).queryByText(/Đang lưu/)).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(state.puts.length).toBeGreaterThanOrEqual(2));
  });
});

function withPaperFinding() {
  return {
    ...baseline(),
    findings: [
      ...baseline().findings,
      {
        id: "f-paper",
        code: "AMBIGUOUS_KEY_PAPER",
        severity: "blocking" as const,
        field: "1,2",
        count: 1,
        evidence: [],
      },
    ],
  };
}

function renderWithDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/imports/:id/review", element: <ImportReviewPage /> },
      { path: "/teacher/imports/:id", element: <p>import detail</p> },
      { path: "/teacher/imports", element: <p>history</p> },
    ],
    { initialEntries: [`/teacher/imports/${IMPORT_ID}/review`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return {
    user: userEvent.setup({ advanceTimers: vi.advanceTimersByTime }),
    client,
    router,
  };
}

async function choosePaper(user: ReturnType<typeof userEvent.setup>) {
  const notice = await screen.findByText("Tệp đáp án có nhiều mã đề");
  const box = notice.closest<HTMLElement>("[id^='finding-']")!;
  await user.click(within(box).getByRole("radio", { name: "Đề số 2" }));
  await user.click(
    within(box).getByRole("button", { name: "Xử lý lại với mã đề này" }),
  );
  return box;
}

describe("a review that retention removed", () => {
  it("says so instead of failing to load", async () => {
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport({ status: "committed", filesRemovedAt: "2026-10-26T00:00:00Z" }),
        ),
      ),
      http.get(`${BASE}/teacher/imports/:id/review`, () =>
        contractJson(
          "/teacher/imports/{id}/review",
          "get",
          410,
          errorBody("IMPORT_FILES_REMOVED", "Bản rà soát đã được xoá."),
        ),
      ),
    );
    renderWithDetail();

    expect(
      await screen.findByText(
        "Bản rà soát của lượt nhập này đã được xoá theo chính sách lưu trữ.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Xem lượt nhập" })).toHaveAttribute(
      "href",
      `/teacher/imports/${IMPORT_ID}`,
    );
  });
});

describe("reprocessing with a chosen key paper", () => {
  it("is not offered while processing is switched off, and the finding says how to finish instead", async () => {
    serveReview(withPaperFinding(), state);
    server.use(
      capabilities(false),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson("/teacher/imports/{id}", "get", 200, wordImport()),
      ),
    );
    renderWithDetail();
    const notice = await screen.findByText("Tệp đáp án có nhiều mã đề");
    const box = notice.closest<HTMLElement>("[id^='finding-']")!;

    expect(
      await within(box).findByText(
        /^Máy chủ đang tắt xử lý tài liệu nên chưa xử lý lại theo một mã đề/,
      ),
    ).toBeInTheDocument();
    expect(
      within(box).queryByRole("button", { name: "Xử lý lại với mã đề này" }),
    ).toBeNull();
  });

  it("withdraws the paper choice when the server refuses to reprocess because processing is off", async () => {
    let processing = true;
    serveReview(withPaperFinding(), state);
    server.use(
      http.get(`${BASE}/teacher/imports/capabilities`, () =>
        contractJson("/teacher/imports/capabilities", "get", 200, {
          intakeEnabled: true,
          processingEnabled: processing,
          retention: { afterCommitDays: 30, afterCancelDays: 7, idleDays: 60 },
        }),
      ),
      http.get(`${BASE}/teacher/imports/:id`, () =>
        contractJson("/teacher/imports/{id}", "get", 200, wordImport()),
      ),
      http.post(`${BASE}/teacher/imports/:id/process`, () => {
        processing = false;
        return contractJson(
          "/teacher/imports/{id}/process",
          "post",
          503,
          errorBody(
            "IMPORT_PROCESSING_UNAVAILABLE",
            "Máy chủ này chưa bật xử lý tài liệu nên chưa thể xử lý lượt nhập.",
          ),
        );
      }),
    );
    const { user } = renderWithDetail();
    const box = await choosePaper(user);

    expect(
      await within(box).findByText(
        /^Máy chủ đang tắt xử lý tài liệu nên chưa xử lý lại theo một mã đề/,
      ),
    ).toBeInTheDocument();
    expect(
      within(box).queryByRole("button", { name: "Xử lý lại với mã đề này" }),
    ).toBeNull();
  });

  it("re-reads the import after a lost reprocess response instead of staying editable", async () => {
    let reads = 0;
    let posts = 0;
    serveReview(withPaperFinding(), state);
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () => {
        reads += 1;
        return contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport(
            posts > 0 ? { status: "queued", run: run({ status: "queued" }) } : {},
          ),
        );
      }),
      http.post(`${BASE}/teacher/imports/:id/process`, () => {
        posts += 1;
        return HttpResponse.error();
      }),
    );
    const { user } = renderWithDetail();
    const box = await choosePaper(user);
    expect(
      await screen.findByText("Không xử lý lại được. Vui lòng thử lại."),
    ).toBeInTheDocument();

    expect(
      await screen.findByText(/^Đang xử lý lại\. Nếu bạn đã chỉnh bản rà soát/),
      "the import is re-read, so a run the server did accept shows as processing",
    ).toBeInTheDocument();
    expect(
      within(box).getByRole("button", { name: "Xử lý lại với mã đề này" }),
    ).toBeDisabled();
    expect(screen.getByLabelText("Tên đề")).toBeDisabled();
    expect(posts).toBe(1);
    expect(reads).toBeGreaterThan(1);
  });

  it("treats a retry that finds the import processing as the earlier request, and marks the cached review stale", async () => {
    let reads = 0;
    let posts = 0;
    serveReview(withPaperFinding(), state);
    server.use(
      capabilities(),
      http.get(`${BASE}/teacher/imports/:id`, () => {
        reads += 1;
        return contractJson(
          "/teacher/imports/{id}",
          "get",
          200,
          wordImport(
            reads >= 4 ? { status: "queued", run: run({ status: "queued" }) } : {},
          ),
        );
      }),
      http.post(`${BASE}/teacher/imports/:id/process`, () => {
        posts += 1;
        return HttpResponse.error();
      }),
    );
    const { user, client } = renderWithDetail();
    const box = await choosePaper(user);
    expect(
      await screen.findByText("Không xử lý lại được. Vui lòng thử lại."),
    ).toBeInTheDocument();
    await waitFor(() => expect(reads).toBe(3));

    await user.click(
      within(box).getByRole("button", { name: "Xử lý lại với mã đề này" }),
    );
    expect(await screen.findByText("import detail")).toBeInTheDocument();
    expect(posts, "the retry sends no second run").toBe(1);
    expect(client.getQueryState(["word-import-review", IMPORT_ID])?.isInvalidated).toBe(
      true,
    );
  });

  it("stays where the teacher went when a reprocess answers after they left", async () => {
    const gate = deferred<void>();
    let posts = 0;
    serveReview(withPaperFinding(), state);
    server.use(
      http.post(`${BASE}/teacher/imports/:id/process`, async () => {
        posts += 1;
        await gate.promise;
        return contractJson(
          "/teacher/imports/{id}/process",
          "post",
          202,
          wordImport({ status: "queued", run: run({ status: "queued" }) }),
        );
      }),
    );
    const { user, router } = renderWithDetail();
    await choosePaper(user);
    await waitFor(() => expect(posts).toBe(1));

    await act(() => router.navigate("/teacher/imports"));
    expect(await screen.findByText("history")).toBeInTheDocument();
    gate.resolve();
    await vi.advanceTimersByTimeAsync(200);
    expect(router.state.location.pathname).toBe("/teacher/imports");
  });
});
