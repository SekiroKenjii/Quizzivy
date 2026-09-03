import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import AssignmentIntroPage from "@/features/assignments/pages/AssignmentIntroPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import {
  ASSIGNMENT,
  ATTEMPT,
  BASE,
  POLICY,
  detail,
  mockStart,
  renderAt,
} from "./support";
import "@/lib/i18n";

function show(over: Record<string, unknown> = {}) {
  server.use(
    http.get(`${BASE}/app/assignments/${ASSIGNMENT}`, () =>
      contractJson("/app/assignments/{id}", "get", 200, detail(over)),
    ),
  );
  return renderAt(`/app/assignments/${ASSIGNMENT}`, [
    { path: "/app/assignments/:id", element: <AssignmentIntroPage /> },
  ]);
}

const rules = async () => {
  const heading = await screen.findByRole("heading", { name: "Khi làm bài" });
  return heading.parentElement!.textContent ?? "";
};

const request = vi.fn<() => Promise<void>>();

beforeEach(() => {
  request.mockReset().mockResolvedValue(undefined);
  document.documentElement.requestFullscreen = request;
  Object.defineProperty(document, "fullscreenEnabled", {
    configurable: true,
    get: () => true,
  });
  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => null,
  });
});
afterEach(() => {
  useAuthStore.getState().clearSession();
});

/**
 * §10.2: rules are announced before the clock starts, from the same sentences
 * the teacher saw in G-01. Four policies, four different lists.
 */
describe("what the intro states, per policy", () => {
  it("§10.3's defaults: copy blocked, no fullscreen, no limit", async () => {
    show();
    const text = await rules();
    expect(text).toContain("45 phút");
    expect(text).toContain("Sao chép và dán bị tắt");
    expect(text).not.toContain("toàn màn hình");
    expect(text).not.toContain("rời khỏi trang");
    expect(text).toContain("lưu tự động");
  });

  it("fullscreen with a flag limit states the consequence and the honest limit", async () => {
    show({ integrity: { ...POLICY, requireFullscreen: true, maxFocusLoss: 2 } });
    const text = await rules();
    expect(text).toContain("toàn màn hình");
    expect(text).toContain("quá 2 lần, bài sẽ được đánh dấu để giáo viên xem lại");
    expect(text).toContain("điểm không bị trừ tự động");
    expect(text).toContain("chỉ ghi nhận việc trang này mất tập trung");
  });

  it("a warn limit with copy allowed says so, and nothing about copying", async () => {
    show({
      integrity: {
        ...POLICY,
        blockCopyPaste: false,
        maxFocusLoss: 3,
        onLimitExceeded: "warn",
      },
    });
    const text = await rules();
    expect(text).toContain("quá 3 lần, bạn sẽ được nhắc");
    expect(text).not.toContain("Sao chép");
  });

  it("auto_submit says the paper is submitted", async () => {
    show({ integrity: { ...POLICY, maxFocusLoss: 1, onLimitExceeded: "auto_submit" } });
    expect(await rules()).toContain("được nộp tự động");
  });

  it("states the listening cap when the paper has audio, and not otherwise", async () => {
    show({ hasAudio: true, audioMaxPlays: 2 });
    expect(await rules()).toContain("Mỗi câu nghe được phát tối đa 2 lần");
  });

  it("says replays are unlimited when they are", async () => {
    show({ hasAudio: true, audioMaxPlays: null });
    expect(await rules()).toContain("không giới hạn");
  });

  it("lists attempts only when there is more than one", async () => {
    show({ maxAttempts: 1 });
    expect(await rules()).not.toContain("tối đa");
  });
});

// A permission list, not a feature list: ticks and crosses in one block.
describe("after submitting", () => {
  it("names what is hidden beside what is shown", async () => {
    show();
    const heading = await screen.findByRole("heading", { name: "Sau khi nộp" });
    const text = heading.parentElement!.textContent ?? "";
    expect(text).toContain("Xem điểm của mình");
    expect(text).toContain("Không xem đáp án đúng");
    expect(text).toContain("Xem giải thích từng câu");
  });
});

describe("starting", () => {
  it("enters fullscreen from the click when required, then opens the paper", async () => {
    const user = userEvent.setup();
    const calls = mockStart();
    const router = show({ integrity: { ...POLICY, requireFullscreen: true } });

    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));
    expect(request).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/app/attempts/${ATTEMPT}`),
    );
    expect(calls).toEqual(["start"]);
  });

  it("does not touch fullscreen when the policy does not ask for it", async () => {
    const user = userEvent.setup();
    mockStart();
    show();
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));
    expect(request).not.toHaveBeenCalled();
  });

  it("says the clock cannot be paused, under the button", async () => {
    show();
    await screen.findByRole("button", { name: "Bắt đầu làm bài" });
    expect(
      screen.getByText("Bấm bắt đầu là đồng hồ chạy. Bạn không thể tạm dừng."),
    ).toBeInTheDocument();
  });

  it("offers resume for a live attempt, without the pause warning", async () => {
    show({ hasLiveAttempt: true, lastAttemptId: ATTEMPT });
    expect(
      await screen.findByRole("button", { name: "Tiếp tục làm bài" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/không thể tạm dừng/)).toBeNull();
  });

  it("shows the server's refusal verbatim", async () => {
    const user = userEvent.setup();
    mockStart(409);
    show();
    await user.click(await screen.findByRole("button", { name: "Bắt đầu làm bài" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bạn đã dùng hết số lượt làm bài.",
    );
  });
});

describe("when there is nothing to start", () => {
  it("says the attempts are spent", async () => {
    show({ attemptsUsed: 2, maxAttempts: 2 });
    expect(
      await screen.findByText("Bạn đã dùng hết số lượt làm bài."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Bắt đầu làm bài" })).toBeNull();
  });

  it("says when a scheduled one opens", async () => {
    show({ status: "scheduled", opensAt: "2026-09-01T01:00:00Z" });
    expect(await screen.findByText("Bài mở lúc 08:00 · 01/09.")).toBeInTheDocument();
  });

  it("says a closed one is closed", async () => {
    show({ status: "closed" });
    expect(await screen.findByText("Bài này đã đóng.")).toBeInTheDocument();
  });
});

/** S-04's header and its fourth fact. */
describe("what the deck's intro states", () => {
  it("names the class and the teacher above the title", async () => {
    show({ className: "IELTS Foundation", teacherName: "Cô Thương" });
    expect(await screen.findByText("IELTS Foundation · Cô Thương")).toBeInTheDocument();
  });

  it("names whichever half the server could name, and neither is not a blank line", async () => {
    show({ className: "IELTS Foundation", teacherName: null });
    expect(await screen.findByText("IELTS Foundation")).toBeInTheDocument();
  });

  it("states the question count and what the paper is out of", async () => {
    show({ questionCount: 24, totalPoints: 30 });
    expect(await screen.findByText("Số câu")).toBeInTheDocument();
    expect(screen.getByText("24 câu · 30 điểm")).toBeInTheDocument();
  });

  it("offers the transcript only when the paper releases one", async () => {
    show({ hasAudio: true, showsTranscript: true });
    expect(await screen.findByText("Xem lời thoại bài nghe")).toBeInTheDocument();
  });

  it("says nothing about transcripts on a paper with no listening question", async () => {
    show({ showsTranscript: false });
    await screen.findByText("Sau khi nộp");
    expect(screen.queryByText("Xem lời thoại bài nghe")).toBeNull();
  });
});
