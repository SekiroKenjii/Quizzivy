import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { Timeline } from "@/features/integrity/components/Timeline";
import { timelineRows, hasOpenEpisode } from "@/features/integrity/timeline";
import type { IntegrityEvent } from "@/features/attempts/api";
import { POLL_MS } from "@/features/attempts/keys";
import { IDLE_AFTER_MS } from "@/hooks/useIdlePolling";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const ATTEMPT_ID = "018f0000-0000-7000-8000-0000000000a7";
const SESSION = "018f0000-0000-7000-8000-00000000ab01";

function event(
  over: Partial<IntegrityEvent> & { id: number; kind: string },
): IntegrityEvent {
  return {
    occurredAt: "2026-09-04T02:15:41Z",
    offsetMs: 341_000,
    clientSeq: over.id,
    sessionId: SESSION,
    ...over,
  };
}

const EVENTS: IntegrityEvent[] = [
  event({
    id: 1,
    kind: "window_blur",
    durationMs: 72_000,
    offsetMs: 459_000,
    occurredAt: "2026-09-04T02:17:41Z",
  }),
  event({ id: 2, kind: "tab_hidden", durationMs: null, offsetMs: 459_200 }),
  event({ id: 3, kind: "tab_visible", offsetMs: 531_000 }),
  event({ id: 4, kind: "window_focus", offsetMs: 531_100 }),
  event({ id: 5, kind: "paste", offsetMs: 600_000 }),
  event({ id: 6, kind: "network_offline", durationMs: 48_000, offsetMs: 1_013_000 }),
  event({ id: 7, kind: "network_online", offsetMs: 1_061_000 }),
  event({ id: 8, kind: "window_blur", durationMs: null, offsetMs: 2_138_000 }),
];

describe("timelineRows", () => {
  it("keeps one row per episode with its duration and drops the returns", () => {
    const rows = timelineRows(EVENTS, "all");
    expect(rows.map((r) => r.event.kind)).toEqual([
      "window_blur",
      "paste",
      "network_offline",
      "window_blur",
    ]);
    expect(rows[0]?.event.durationMs).toBe(72_000);
    expect(rows[0]?.ongoing).toBe(false);
  });

  it("marks an unpaired trailing leave as ongoing rather than dropping it", () => {
    const rows = timelineRows(EVENTS, "away");
    expect(rows).toHaveLength(2);
    expect(rows[1]?.ongoing).toBe(true);
    expect(hasOpenEpisode(EVENTS)).toBe(true);
    expect(hasOpenEpisode(EVENTS.slice(0, 7))).toBe(false);
  });
});

function renderTimeline(
  events: IntegrityEvent[],
  summary = {
    totalAwayMs: 72_000,
    awayEpisodes: 2,
    pasteCount: 1,
    resumeCount: 0,
    audioReplays: 0,
    offlineEpisodes: 1,
  },
) {
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt: "2026-09-04T02:10:00Z",
        events,
        summary,
      }),
    ),
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Timeline
        attemptId={ATTEMPT_ID}
        questions={[]}
        live={false}
        note={null}
        onViewPaper={() => {}}
      />
    </QueryClientProvider>,
  );
}

describe("the integrity timeline", () => {
  it("renders a paired episode with its duration and a trailing one as open-ended", async () => {
    renderTimeline(EVENTS);
    const table = await screen.findByRole("table");
    expect(within(table).getByText("1:12")).toBeInTheDocument();
    expect(screen.getByText("— đang tiếp diễn")).toBeInTheDocument();
    expect(screen.getByText("chưa tính lần đang mở")).toBeInTheDocument();
    expect(screen.getByText("Mất kết nối")).toBeInTheDocument();
  });

  it("carries no destructive or red semantic class anywhere (§12)", async () => {
    const { container } = renderTimeline(EVENTS);
    await screen.findByRole("table");
    // A class that paints red, not a pseudo-variant the primitives carry for invalid fields.
    const paintsRed = (token: string) =>
      /^(bg|text|border|ring)-(destructive|danger)/.test(token) ||
      token.includes("var(--destructive)");
    const red = [...container.querySelectorAll("*")].filter(
      (el) =>
        el.getAttribute("data-variant") === "destructive" ||
        (el.getAttribute("class") ?? "").split(/\s+/).some(paintsRed),
    );
    expect(red).toHaveLength(0);
    expect(container.textContent).not.toMatch(/gian lận|vi phạm/i);
  });

  it("shows — rather than 0 when nothing was recorded", async () => {
    renderTimeline([], {
      totalAwayMs: 0,
      awayEpisodes: 0,
      pasteCount: 0,
      resumeCount: 0,
      audioReplays: 0,
      offlineEpisodes: 0,
    });
    expect(
      await screen.findByText("Không ghi nhận sự kiện nào trong lượt làm này."),
    ).toBeInTheDocument();
    expect(screen.getAllByText("—")).toHaveLength(6);
  });
});

describe("audio plays", () => {
  const QUESTION = "018f0000-0000-7000-8000-0000000000c1";

  function audio(
    id: number,
    kind: "audio_play" | "audio_ended" | "audio_blocked",
    durationMs: number | null = null,
  ): IntegrityEvent {
    return event({ id, kind, questionId: QUESTION, durationMs });
  }

  it("shows a finished play with its duration and hides the end", () => {
    const rows = timelineRows(
      [audio(1, "audio_play", 7000), audio(2, "audio_ended")],
      "all",
    );
    expect(rows).toHaveLength(1);
    expect(rows[0]?.event.kind).toBe("audio_play");
    expect(rows[0]?.event.durationMs).toBe(7000);
    expect(rows[0]?.ongoing).toBe(false);
    expect(rows[0]?.playNo).toBe(1);
  });

  it("keeps a play that never ended, without calling it ongoing", () => {
    const rows = timelineRows([audio(1, "audio_play")], "all");
    expect(rows).toHaveLength(1);
    expect(rows[0]?.ongoing).toBe(false);
  });

  it("keeps a play stopped early when a later play of the same question ended", () => {
    const rows = timelineRows(
      [audio(1, "audio_play"), audio(2, "audio_play", 7000), audio(3, "audio_ended")],
      "all",
    );
    expect(rows.map((row) => row.playNo)).toEqual([1, 2]);
    expect(rows.map((row) => row.ongoing)).toEqual([false, false]);
  });

  it("shows a blocked play as its own row", () => {
    const rows = timelineRows(
      [audio(1, "audio_play"), audio(2, "audio_blocked")],
      "all",
    );
    expect(rows).toHaveLength(2);
    expect(rows[1]?.event.kind).toBe("audio_blocked");
  });

  it("leaves an away episode ongoing as before", () => {
    const rows = timelineRows(EVENTS, "all");
    expect(rows.at(-1)?.event.kind).toBe("window_blur");
    expect(rows.at(-1)?.ongoing).toBe(true);
  });

  it("draws a play that never ended with no duration and no open-ended mark", async () => {
    renderTimeline([audio(1, "audio_play")]);
    const row = await screen.findByRole("row", { name: /Phát âm thanh · lần 1/ });
    expect(within(row).getAllByRole("cell")[3]).toHaveTextContent(/^—$/);
    expect(screen.queryByText(/đang tiếp diễn/)).not.toBeInTheDocument();
  });
});

describe("the integrity timeline's poll", () => {
  let fetches = 0;

  function renderPolled(live: boolean) {
    fetches = 0;
    server.use(
      http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () => {
        fetches += 1;
        return contractJson("/teacher/attempts/{id}/events", "get", 200, {
          startedAt: "2026-09-04T02:10:00Z",
          events: EVENTS,
          summary: {
            totalAwayMs: 72_000,
            awayEpisodes: 2,
            pasteCount: 1,
            resumeCount: 0,
            audioReplays: 0,
            offlineEpisodes: 1,
          },
        });
      }),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <Timeline
          attemptId={ATTEMPT_ID}
          questions={[]}
          live={live}
          note={null}
          onViewPaper={() => {}}
        />
      </QueryClientProvider>,
    );
    return client;
  }

  function advance(ms: number) {
    return act(() => vi.advanceTimersByTimeAsync(ms));
  }

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("polls a live attempt every 15s, stops after ten idle minutes, refetches at once on a key press and polls from there", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const client = renderPolled(true);
    await screen.findByRole("table");
    expect(fetches).toBe(1);

    await advance(POLL_MS + 100);
    await waitFor(() => expect(fetches).toBe(2));

    await advance(IDLE_AFTER_MS);
    await waitFor(() => expect(client.isFetching()).toBe(0));
    const whenIdle = fetches;
    await advance(5 * 60_000);
    expect(client.isFetching()).toBe(0);
    expect(fetches).toBe(whenIdle);

    const pressedAt = Date.now();
    fireEvent.keyDown(window, { key: "Shift" });
    expect(client.isFetching()).toBe(1);
    await waitFor(() => expect(client.isFetching()).toBe(0));
    expect(fetches).toBe(whenIdle + 1);
    expect(Date.now() - pressedAt).toBeLessThan(POLL_MS);

    await advance(12_000);
    expect(fetches).toBe(whenIdle + 1);
    await advance(3_100);
    await waitFor(() => expect(fetches).toBe(whenIdle + 2));
  });

  it("sends nothing on a key press once the attempt is over", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const client = renderPolled(false);
    await screen.findByRole("table");

    await advance(IDLE_AFTER_MS);
    fireEvent.keyDown(window, { key: "Shift" });
    expect(client.isFetching()).toBe(0);
    await advance(31_000);
    expect(fetches).toBe(1);
  });
});
