import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { http } from "msw";
import { FocusLossCell } from "@/features/attempts/components/FocusLossCell";
import { Timeline } from "@/features/integrity/components/Timeline";
import {
  FLAGGED,
  TIMELINE_DOT,
  focusLossLabel,
  timelineMark,
} from "@/features/integrity/tones";
import { KIND_LOOK } from "@/features/notifications/kinds";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { ATTEMPT_ID, BASE, review } from "../attempts/fixtures";
import "@/lib/i18n";

describe("the flagged tone", () => {
  it("is the danger tokens, and the flagged notification reads it", () => {
    expect(FLAGGED).toEqual({
      tone: "danger",
      ink: "text-danger-ink",
      soft: "bg-danger-soft",
    });
    expect(KIND_LOOK["attempt.flagged"].tone).toBe(FLAGGED.tone);
  });
});

describe("the timeline's dots", () => {
  it("are info for the start, danger for leaving, success for submitted and border for autosave", () => {
    expect(TIMELINE_DOT).toEqual({
      start: "bg-info",
      away: "bg-danger",
      submitted: "bg-success",
      autosave: "bg-border",
      other: "bg-muted-fg",
    });
  });

  it.each(["tab_hidden", "window_blur", "fullscreen_exit", "auto_submit"])(
    "marks %s as leaving",
    (kind) => expect(timelineMark(kind)).toBe("away"),
  );

  it.each(["network_offline", "audio_play", "paste", "resume", "session_takeover"])(
    "keeps %s neutral, so it never reads as a focus loss",
    (kind) => expect(timelineMark(kind)).toBe("other"),
  );
});

describe("the roster's focus-loss value", () => {
  it.each([
    [null, "—"],
    [undefined, "—"],
    [0, "—"],
    [1, "1×"],
    [14, "14×"],
  ] as const)("reads %s as %s", (count, label) => {
    expect(focusLossLabel(count)).toBe(label);
  });

  it("is danger ink with a flag once flagged, and muted without one otherwise", () => {
    const { container, rerender } = render(<FocusLossCell count={3} flagged />);
    const cell = container.firstElementChild!;
    expect(cell).toHaveTextContent(/^3×$/);
    expect(cell).toHaveClass(FLAGGED.ink);
    expect(cell.querySelector("svg")).toHaveAttribute("aria-hidden", "true");

    rerender(<FocusLossCell count={2} flagged={false} />);
    expect(container.firstElementChild).toHaveTextContent(/^2×$/);
    expect(container.firstElementChild).toHaveClass("text-muted-fg");
    expect(container.querySelector("svg")).toBeNull();
  });
});

function renderCompact(submittedAt: string | null, events: unknown[]) {
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt: "2026-09-04T02:10:00Z",
        summary: review().integrity,
        events,
      }),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <Timeline
        attemptId={ATTEMPT_ID}
        questions={[]}
        live={false}
        note={null}
        submittedAt={submittedAt}
        presentation="compact"
        onViewPaper={() => {}}
      />
    </QueryClientProvider>,
  );
}

function dotOf(item: HTMLElement) {
  return item.querySelector("span[aria-hidden='true']");
}

describe("the compact timeline", () => {
  it("runs from the start through autosave to the submission when nothing was recorded", async () => {
    renderCompact("2026-09-04T02:40:00Z", []);
    const region = await screen.findByRole("region", { name: "Diễn biến bài làm" });
    const items = within(region).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent(/^Bắt đầu làm bài/);
    expect(items[1]).toHaveTextContent(
      /^Câu trả lời được lưu tự độngtrong lúc làm bài$/,
    );
    expect(items[2]).toHaveTextContent(/^Nộp bài/);
    expect(items.map((item) => dotOf(item)?.className)).toEqual([
      expect.stringContaining(TIMELINE_DOT.start),
      expect.stringContaining(TIMELINE_DOT.autosave),
      expect.stringContaining(TIMELINE_DOT.submitted),
    ]);
  });

  it("says how long a student left, in seconds, and ends without a submission while one is not in", async () => {
    renderCompact(null, [
      {
        id: 1,
        sessionId: "018f0000-0000-7000-8000-00000000ab01",
        clientSeq: 1,
        kind: "fullscreen_exit",
        offsetMs: 10_000,
        occurredAt: "2026-09-04T02:10:10Z",
        durationMs: 14_000,
      },
      {
        id: 2,
        sessionId: "018f0000-0000-7000-8000-00000000ab01",
        clientSeq: 2,
        kind: "fullscreen_enter",
        offsetMs: 24_000,
        occurredAt: "2026-09-04T02:10:24Z",
      },
      {
        id: 3,
        sessionId: "018f0000-0000-7000-8000-00000000ab01",
        clientSeq: 3,
        kind: "tab_hidden",
        offsetMs: 60_000,
        occurredAt: "2026-09-04T02:11:00Z",
        durationMs: 120_000,
      },
      {
        id: 4,
        sessionId: "018f0000-0000-7000-8000-00000000ab01",
        clientSeq: 4,
        kind: "tab_visible",
        offsetMs: 180_000,
        occurredAt: "2026-09-04T02:13:00Z",
      },
    ]);
    const region = await screen.findByRole("region", { name: "Diễn biến bài làm" });
    const items = within(region).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items[1]).toHaveTextContent("Thoát toàn màn hình · 14 giây");
    expect(items[2]).toHaveTextContent("Chuyển sang tab khác · 2 phút");
    expect(dotOf(items[1]!)).toHaveClass(TIMELINE_DOT.away);
    expect(dotOf(items[2]!)).toHaveClass(TIMELINE_DOT.away);
    expect(within(region).queryByText("Nộp bài")).toBeNull();
    expect(within(region).queryByText("Câu trả lời được lưu tự động")).toBeNull();
  });
});
