import { afterEach, expect, it } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { Timeline } from "@/features/integrity/components/Timeline";
import { setDisplayTimeZone, APP_TIME_ZONE } from "@/lib/i18n/datetime";
import { server } from "@tests/support/server";
import "@/lib/i18n";

afterEach(() => {
  cleanup();
  setDisplayTimeZone(APP_TIME_ZONE);
});

it("changes a mounted event clock immediately while retaining episode duration and row identity", async () => {
  server.use(
    http.get("http://localhost:8080/teacher/attempts/a/events", () =>
      HttpResponse.json({
        startedAt: "2026-03-08T06:00:00Z",
        events: [
          {
            id: 1,
            kind: "window_blur",
            occurredAt: "2026-03-08T06:30:41Z",
            durationMs: 72000,
            offsetMs: 341000,
            clientSeq: 1,
            sessionId: "s",
          },
        ],
        summary: {
          totalAwayMs: 72000,
          awayEpisodes: 1,
          pasteCount: 0,
          resumeCount: 0,
          audioReplays: 0,
          offlineEpisodes: 0,
        },
      }),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <Timeline
        attemptId="a"
        questions={[]}
        live={false}
        note={null}
        onViewPaper={() => undefined}
      />
    </QueryClientProvider>,
  );
  const clock = await screen.findByText("13:30:41");
  const row = clock.closest("tr");
  act(() => {
    setDisplayTimeZone("America/New_York");
  });
  expect(screen.getByText("01:30:41").closest("tr")).toBe(row);
  expect(within(row!).getByText("1:12")).toBeInTheDocument();
  act(() => {
    setDisplayTimeZone("UTC");
  });
  expect(screen.getByText("06:30:41").closest("tr")).toBe(row);
});
