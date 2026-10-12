import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { server } from "@tests/support/server";
import { http } from "msw";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, finding, question, review, section } from "./fixtures";
import {
  KEY_EVIDENCE,
  baseline,
  card,
  cardHeader,
  isOpen,
  renderReview,
  serveReview,
  type ReviewServer,
} from "./reviewHarness";
import "@/lib/i18n";

let state: ReviewServer;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
  serveReview(baseline(), state);
});

afterEach(() => {
  vi.useRealTimers();
});

function editorFor(id: string) {
  expect(isOpen(id), `card ${id} is open`).toBe(true);
  return card(id);
}

function savedQuestion(index: number, id: string) {
  const items = state.puts[index]!.sections[0]!.items;
  return items.find((item) => item.question?.id === id)!.question!;
}

describe("resolving findings in the review", () => {
  it("opens on the first question that needs a decision, with its source text linked", async () => {
    await renderReview();
    expect(isOpen("q1")).toBe(true);
    const block = await screen.findByRole("button", { name: /to school yesterday/ });
    expect(block).toHaveAttribute("aria-pressed", "true");
    expect(
      block.querySelector("u"),
      "the source keeps its underline",
    ).toBeInTheDocument();
  });

  it("opens the question a source block became when the block is clicked", async () => {
    const { user } = await renderReview();
    await user.click(
      await screen.findByRole("button", { name: /Which colour is the sky/ }),
    );
    expect(isOpen("q2")).toBe(true);

    await user.click(screen.getByRole("button", { name: "Đáp án" }));
    await user.click(await screen.findByRole("button", { name: /1\. B/ }));
    expect(isOpen("q1"), "a key line opens the question it answers").toBe(true);
  });

  it("settles a conflicting key on the chosen value and records it as the teacher's", async () => {
    const { user } = await renderReview();
    const editor = editorFor("q1");
    await user.click(
      within(editor).getByRole("radio", { name: "Dùng giá trị này: B" }),
    );

    expect(
      within(editor).getByRole("radio", { name: "Lựa chọn B là đáp án đúng" }),
    ).toBeChecked();
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts).toHaveLength(1));
    const saved = savedQuestion(0, "q1");
    expect(saved.answer.state).toBe("known");
    expect(saved.answer.optionIds).toEqual(["q1-b"]);
    expect(saved.answer.candidates).toBeUndefined();
    expect(saved.answer.evidence, "the chosen value's evidence is kept").toEqual([
      KEY_EVIDENCE,
    ]);
    expect(saved.origins.answer).toBe("teacher_entered");
    expect(saved.origins.prompt, "untouched fields keep their provenance").toBe(
      "source_explicit",
    );
  });

  it("offers acknowledgement only for a review item, never for a blocker", async () => {
    const { user } = await renderReview();
    await user.click(cardHeader("q2"));
    const editor = editorFor("q2");

    const blocker = editor.querySelector<HTMLElement>("#finding-f-missing")!;
    const review = editor.querySelector<HTMLElement>("#finding-f-irregular")!;
    expect(within(blocker).queryByRole("button", { name: /Xác nhận/ })).toBeNull();
    expect(
      within(review).getByRole("button", { name: "Xác nhận đã kiểm tra" }),
    ).toBeInTheDocument();
    expect(
      within(editor).getAllByRole("button", { name: "Xác nhận đã kiểm tra" }),
    ).toHaveLength(1);

    await user.click(
      within(review).getByRole("button", { name: "Xác nhận đã kiểm tra" }),
    );
    expect(within(review).getByText("Đã xác nhận")).toBeInTheDocument();
    expect(within(review).getByRole("button", { name: "Đổi lại" })).toBeInTheDocument();

    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts).toHaveLength(1));
    expect(state.puts[0]!.acknowledged).toEqual(["f-irregular"]);
  });

  it("walks the open items with Previous and Next, and narrows them with the filter", async () => {
    const { user } = await renderReview();
    const next = screen.getByRole("button", { name: "Tiếp" });
    const previous = screen.getByRole("button", { name: "Mục trước cần rà soát" });
    expect(screen.getByText("3 mục còn mở")).toBeInTheDocument();

    await user.click(next);
    expect(isOpen("q1")).toBe(true);
    expect(screen.getByText("1 / 3 mục còn mở")).toBeInTheDocument();
    await waitFor(() => expect(document.activeElement?.id).toBe("finding-f-conflict"));

    await user.click(next);
    expect(isOpen("q2")).toBe(true);
    expect(screen.getByText("2 / 3 mục còn mở")).toBeInTheDocument();
    await waitFor(() => expect(document.activeElement?.id).toBe("finding-f-missing"));

    await user.click(previous);
    expect(isOpen("q1")).toBe(true);

    await user.click(screen.getByRole("button", { name: "Cần xác nhận 1" }));
    await user.click(next);
    expect(isOpen("q2")).toBe(true);
    expect(screen.getByText("1 / 1 mục còn mở")).toBeInTheDocument();
    expect(
      document.querySelector('[data-card="q1"]'),
      "a question outside the filter is not listed",
    ).toBeNull();
  });

  it("keeps processing notes out of the navigation, in the whole-test card and under Info", async () => {
    const { user } = await renderReview();
    expect(screen.queryByText("2 câu đang dùng điểm mặc định")).toBeNull();
    await user.click(cardHeader("whole-test"));
    expect(screen.getByText("2 câu đang dùng điểm mặc định")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Thông tin 1" }));
    expect(document.querySelector('[data-card="q1"]')).toBeNull();
    expect(screen.getByText("1 mục còn mở")).toBeInTheDocument();
  });

  it("offers the answer key's paper numbers when the key holds several", async () => {
    const draft = review(
      [section([question({ id: "q1", label: "1" })])],
      [
        finding({
          id: "f-paper",
          code: "AMBIGUOUS_KEY_PAPER",
          severity: "blocking",
          field: "101,102",
        }),
      ],
    );
    server.use(
      http.get(`${BASE}/teacher/imports/:id/review`, () =>
        contractJson("/teacher/imports/{id}/review", "get", 200, draft),
      ),
    );
    await renderReview();
    const notice = document.getElementById("finding-f-paper")!;
    expect(
      within(notice).getByRole("radio", { name: "Đề số 101" }),
    ).toBeInTheDocument();
    expect(
      within(notice).getByRole("radio", { name: "Đề số 102" }),
    ).toBeInTheDocument();
    expect(
      within(notice).getByRole("button", { name: "Xử lý lại với mã đề này" }),
    ).toBeDisabled();
  });
});
