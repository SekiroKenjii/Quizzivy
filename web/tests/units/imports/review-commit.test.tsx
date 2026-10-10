import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import type { SaveImportReview } from "@/features/imports/api";
import { BASE, IMPORT_ID, question, review, section, summary } from "./fixtures";
import {
  baseline,
  isOpen,
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

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
});

afterEach(() => {
  vi.useRealTimers();
});

describe("leaving the review for Preview and create", () => {
  it("saves a pending edit, then opens Preview and create", async () => {
    serveReview(baseline(), state);
    const { user, router } = await renderReview();
    await user.type(screen.getByLabelText("Tên đề"), "A");
    await user.click(
      screen.getAllByRole("button", { name: "Xem trước và hoàn tất" })[0]!,
    );

    expect(await screen.findByText("confirm page")).toBeInTheDocument();
    expect(state.puts.at(-1)?.title).toBe("Đề thi học kỳ 1A");
    expect(router.state.location.pathname).toBe(
      `/teacher/imports/${IMPORT_ID}/confirm`,
    );
  });

  it("stays on the review with the leave dialog when the edit cannot be saved", async () => {
    serveReview(baseline(), state);
    server.use(
      http.put(`${BASE}/teacher/imports/:id/review`, async ({ request }) => {
        state.puts.push((await request.json()) as SaveImportReview);
        return new Response(null, { status: 503 });
      }),
    );
    const { user, router } = await renderReview();
    await user.type(screen.getByLabelText("Tên đề"), "A");
    await user.click(
      screen.getAllByRole("button", { name: "Xem trước và hoàn tất" })[0]!,
    );

    const dialog = await screen.findByRole("dialog", { name: "Rời trang rà soát?" });
    expect(within(dialog).getByText(/Chưa lưu được thay đổi/)).toBeInTheDocument();
    expect(screen.queryByText("confirm page")).toBeNull();
    expect(router.state.location.pathname).toBe(`/teacher/imports/${IMPORT_ID}/review`);
  });
});

describe("arriving from Preview and create with a filter", () => {
  it("opens on the filter's first open finding and focuses it", async () => {
    serveReview(baseline(), state);
    await renderReview("?filter=review");

    await waitFor(() => expect(document.activeElement?.id).toBe("finding-f-irregular"));
    expect(isOpen("q2")).toBe(true);
    expect(screen.getByText("1 / 1 mục còn mở")).toBeInTheDocument();
  });

  it("moves focus only once, not when the review renders again", async () => {
    serveReview(baseline(), state);
    const { user } = await renderReview("?filter=blocking");
    await waitFor(() => expect(document.activeElement?.id).toBe("finding-f-conflict"));

    const title = screen.getByLabelText("Tên đề");
    await user.click(title);
    await user.type(title, "A");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts).toHaveLength(1));
    await vi.advanceTimersByTimeAsync(3000);
    expect(title).toHaveFocus();
  });
});

describe("a newer processing result", () => {
  it("adopts a newer processing result only after the teacher confirms losing their edits", async () => {
    serveReview(review(ready().draft.sections, [], { reprocessed: true }), state);
    const adopted: number[] = [];
    server.use(
      http.post(`${BASE}/teacher/imports/:id/review/adopt`, async ({ request }) => {
        adopted.push(
          ((await request.json()) as { expectedRevision: number }).expectedRevision,
        );
        return contractJson(
          "/teacher/imports/{id}/review/adopt",
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
