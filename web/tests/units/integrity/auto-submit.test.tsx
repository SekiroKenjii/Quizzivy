import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AutoSubmitNotice } from "@/features/integrity/components/AutoSubmitNotice";
import { useIntegrityAutoSubmit } from "@/features/integrity/useIntegrityAutoSubmit";
import { useTakeTestStore } from "@/features/take-test/store";
import { saveAnswers, submitAttempt } from "@/features/take-test/api";
import i18n from "@/lib/i18n";
import { session, text } from "../take-test/support";

vi.mock("@/features/take-test/api", () => ({
  submitAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  getAttempt: vi.fn(),
}));

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-09-01T08:00:00Z"));
  useTakeTestStore.getState().reset();
  vi.mocked(submitAttempt).mockReset();
});
afterEach(() => {
  useTakeTestStore.getState().reset();
  vi.useRealTimers();
});

function start(limit: number) {
  const payload = session({
    serverTime: "2026-09-01T08:00:00Z",
    deadlineAt: "2026-09-01T09:00:00Z",
  });
  payload.integrity = {
    ...payload.integrity,
    maxFocusLoss: limit,
    onLimitExceeded: "auto_submit",
  };
  vi.mocked(submitAttempt).mockResolvedValue({
    ...payload.attempt,
    status: "submitted",
    submittedAt: "2026-09-01T08:00:01Z",
  });
  useTakeTestStore.getState().hydrate(payload);
}

it("submits immediately only after the allowed count is exceeded", async () => {
  start(1);
  const view = renderHook(({ count }) => useIntegrityAutoSubmit(count), {
    initialProps: { count: 1 },
  });
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(submitAttempt).not.toHaveBeenCalled();
  view.rerender({ count: 2 });
  expect(view.result.current).toBe(true);
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(submitAttempt).toHaveBeenCalledExactlyOnceWith("att-1", {
    reason: "auto_submit",
  });
  expect(useTakeTestStore.getState().submitState).toBe("done");
});

it("does not submit for an unlimited policy", async () => {
  start(0);
  const view = renderHook(() => useIntegrityAutoSubmit(100));
  await act(() => vi.advanceTimersByTimeAsync(0));
  expect(view.result.current).toBe(false);
  expect(submitAttempt).not.toHaveBeenCalled();
});

describe("the notice that replaces the paper", () => {
  const TITLE = "Bài làm đang được nộp";
  const BODY =
    "Bạn đã rời trang làm bài quá số lần được phép nên bài đang được nộp. Câu trả lời của bạn được giữ lại để chấm. Giáo viên đã được báo.";
  const SENDING = "Đang gửi câu trả lời và nộp bài…";
  const WAITING =
    "Chưa xác nhận được việc nộp bài. Câu trả lời được giữ trên thiết bị và sẽ tự gửi lại. Vui lòng giữ trang này mở.";
  const retry = () => screen.getByRole("button", { name: "Thử lưu lại" });

  afterEach(async () => {
    vi.mocked(saveAnswers).mockReset();
    await act(() => i18n.changeLanguage("vi"));
  });

  it("says the test is being submitted and the answers are kept, on the focus dialog's tile", () => {
    start(1);
    render(<AutoSubmitNotice />);

    expect(screen.getByRole("heading", { level: 1, name: TITLE })).toHaveClass(
      "text-xl",
      "font-semibold",
    );
    expect(screen.getByText(BODY)).toHaveClass("text-muted-fg", "text-base");
    const tile = screen.getByRole("main").querySelector("svg")?.parentElement;
    expect(tile).toHaveClass("size-12", "rounded-xl", "bg-warning-soft");
    expect(tile?.querySelector("svg")).toHaveClass("lucide-eye-off");
    expect(screen.getByRole("main").firstElementChild).toHaveClass(
      "max-w-120",
      "items-center",
      "text-center",
    );
  });

  it("never calls it a violation, in either language", async () => {
    start(1);
    render(<AutoSubmitNotice />);
    expect(screen.getByRole("main")).not.toHaveTextContent(/vi phạm|gian lận/i);

    await act(() => i18n.changeLanguage("en"));
    expect(
      screen.getByRole("heading", { name: "Your test is being submitted" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("main")).toHaveTextContent(
      "You left the test more times than allowed, so it is being submitted. Your answers are kept for grading. Your teacher has been told.",
    );
    expect(screen.getByRole("main")).not.toHaveTextContent(/violat|cheat/i);
  });

  it("says it is sending while the submission is out, with the button inert", async () => {
    start(1);
    let land!: () => void;
    vi.mocked(submitAttempt).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          land = () =>
            resolve(
              session({
                serverTime: "2026-09-01T08:00:01Z",
                deadlineAt: "2026-09-01T09:00:00Z",
                status: "submitted",
              }).attempt,
            );
        }),
    );
    render(<AutoSubmitNotice />);
    expect(screen.getByRole("status")).toHaveTextContent(WAITING);
    expect(retry()).toBeEnabled();

    fireEvent.click(retry());
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(screen.getByRole("status")).toHaveTextContent(SENDING);
    expect(retry()).toBeDisabled();
    expect(retry().querySelector("svg")).toHaveClass("animate-spin");

    fireEvent.click(retry());
    await act(async () => land());
    expect(submitAttempt).toHaveBeenCalledExactlyOnceWith("att-1", {
      reason: "auto_submit",
    });
    expect(useTakeTestStore.getState().submitState).toBe("done");
  });

  it("keeps the answers and waits to send again when the submission cannot go out", async () => {
    start(1);
    vi.mocked(saveAnswers).mockRejectedValue(new Error("offline"));
    act(() => useTakeTestStore.getState().setAnswer("q1", text("final answer")));
    render(<AutoSubmitNotice />);

    fireEvent.click(retry());
    await act(() => vi.advanceTimersByTimeAsync(0));

    expect(submitAttempt).not.toHaveBeenCalled();
    expect(screen.getByRole("status")).toHaveTextContent(WAITING);
    expect(retry()).toBeEnabled();
    expect(useTakeTestStore.getState().answers.q1).toEqual(text("final answer"));
    expect(useTakeTestStore.getState().dirty.has("q1")).toBe(true);
  });
});
