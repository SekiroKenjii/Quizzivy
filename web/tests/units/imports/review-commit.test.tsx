import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  BASE,
  TEST_ID,
  deferred,
  question,
  review,
  section,
  summary,
  wordImport,
} from "./fixtures";
import {
  baseline,
  renderReview,
  serveReview,
  type ReviewServer,
} from "./reviewHarness";
import "@/lib/i18n";

let state: ReviewServer;

function ready() {
  return review([section([question({ id: "q1", label: "1" })])], [], {
    ready: true,
    summary: summary({
      questions: 1,
      included: 1,
      totalPoints: "1.00",
      answersKnown: 1,
      answersMissing: 0,
      blocking: 0,
    }),
  });
}

function committed() {
  return contractJson("/admin/imports/{id}/commit", "post", 200, {
    testId: TEST_ID,
    import: wordImport({ status: "committed", testId: TEST_ID, revision: 7 }),
  });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
});

afterEach(() => {
  vi.useRealTimers();
});

describe("creating the draft test from a review", () => {
  it("keeps Tạo bản nháp đề disabled until the saved review is ready, and links each open item", async () => {
    serveReview(baseline(), state);
    const { user } = await renderReview();
    await user.click(
      screen.getByRole("button", { name: "Xem tóm tắt & tạo bản nháp" }),
    );
    const dialog = await screen.findByRole("dialog");

    expect(
      within(dialog).getByRole("button", { name: "Tạo bản nháp đề" }),
    ).toBeDisabled();
    await user.click(
      within(dialog).getByRole("button", { name: "Xem 2 mục cần xử lý" }),
    );

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("button", { name: "Cần xử lý (2)" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText("Mục 1/2")).toBeInTheDocument();
    await waitFor(() => expect(document.activeElement?.id).toBe("finding-f-conflict"));
  });

  it("returns focus to the summary button when the summary closes", async () => {
    serveReview(baseline(), state);
    const { user } = await renderReview();
    const open = screen.getByRole("button", { name: "Xem tóm tắt & tạo bản nháp" });
    await user.click(open);
    await screen.findByRole("dialog");
    await user.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(open).toHaveFocus();
  });

  it("retries a lost commit with the same request, and names the draft it created", async () => {
    serveReview(ready(), state);
    let attempts = 0;
    server.use(
      http.post(`${BASE}/admin/imports/:id/commit`, async ({ request }) => {
        state.commits.push((await request.json()) as ReviewServer["commits"][number]);
        attempts += 1;
        if (attempts === 1) return HttpResponse.error();
        return committed();
      }),
    );
    const { user } = await renderReview();
    await user.click(
      screen.getByRole("button", { name: "Xem tóm tắt & tạo bản nháp" }),
    );
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByRole("button", { name: "Tạo bản nháp đề" }));

    expect(
      await within(dialog).findByText(/Thử lại sẽ không tạo đề trùng/),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Thử lại" }));

    expect(await within(dialog).findByText("Đã tạo bản nháp đề")).toBeInTheDocument();
    expect(
      within(dialog).getByRole("link", { name: "Mở trình soạn đề" }),
    ).toHaveAttribute("href", `/admin/tests/${TEST_ID}/edit`);
    expect(state.commits).toHaveLength(2);
    expect(
      state.commits[1]!.requestId,
      "a lost response replays the same request",
    ).toBe(state.commits[0]!.requestId);
    expect(state.commits[1]!.draftRevision).toBe(1);
  });

  it("ignores a second click while the commit is pending", async () => {
    serveReview(ready(), state);
    const gate = deferred<void>();
    server.use(
      http.post(`${BASE}/admin/imports/:id/commit`, async ({ request }) => {
        state.commits.push((await request.json()) as ReviewServer["commits"][number]);
        await gate.promise;
        return committed();
      }),
    );
    const { user } = await renderReview();
    await user.click(
      screen.getByRole("button", { name: "Xem tóm tắt & tạo bản nháp" }),
    );
    const dialog = await screen.findByRole("dialog");
    const commit = within(dialog).getByRole("button", { name: "Tạo bản nháp đề" });
    await user.click(commit);
    await waitFor(() => expect(state.commits).toHaveLength(1));
    expect(within(dialog).getByRole("button", { name: "Đang tạo…" })).toBeDisabled();
    await user.click(within(dialog).getByRole("button", { name: "Đang tạo…" }));

    gate.resolve();
    expect(await within(dialog).findByText("Đã tạo bản nháp đề")).toBeInTheDocument();
    expect(state.commits).toHaveLength(1);
  });

  it("adopts a newer processing result only after the teacher confirms losing their edits", async () => {
    serveReview(review(ready().draft.sections, [], { reprocessed: true }), state);
    const adopted: number[] = [];
    server.use(
      http.post(`${BASE}/admin/imports/:id/review/adopt`, async ({ request }) => {
        adopted.push(
          ((await request.json()) as { expectedRevision: number }).expectedRevision,
        );
        return contractJson(
          "/admin/imports/{id}/review/adopt",
          "post",
          200,
          review(ready().draft.sections, [], { revision: 5, reprocessed: false }),
        );
      }),
    );
    const { user } = await renderReview();
    expect(screen.getByText(/Có kết quả xử lý mới hơn/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Dùng kết quả mới" }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/sẽ bị bỏ và không khôi phục được/),
    ).toBeInTheDocument();
    expect(adopted, "nothing is adopted before the confirmation").toEqual([]);

    await user.click(within(dialog).getByRole("button", { name: "Dùng kết quả mới" }));
    await waitFor(() => expect(adopted).toEqual([1]));
    await waitFor(() =>
      expect(screen.queryByText(/Có kết quả xử lý mới hơn/)).toBeNull(),
    );
  });
});
