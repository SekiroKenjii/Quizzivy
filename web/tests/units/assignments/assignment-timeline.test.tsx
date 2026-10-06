import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { expect, it } from "vitest";
import { http } from "msw";
import { Timeline } from "@/features/integrity/components/Timeline";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { ATTEMPT_ID, BASE, review } from "../attempts/fixtures";
import "@/lib/i18n";

it("compact teacher timeline keeps neutral paired durations without the full-review autosave note", async () => {
  const sessionId = "018f0000-0000-7000-8000-00000000ab01";
  const startedAt = "2026-09-04T02:10:00Z";
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt,
        summary: review().integrity,
        events: [
          {
            id: 1,
            sessionId,
            clientSeq: 1,
            kind: "window_blur",
            offsetMs: 10_000,
            occurredAt: "2026-09-04T02:10:10Z",
            durationMs: 72_000,
          },
          {
            id: 2,
            sessionId,
            clientSeq: 2,
            kind: "window_focus",
            offsetMs: 82_000,
            occurredAt: "2026-09-04T02:11:22Z",
          },
          {
            id: 3,
            sessionId,
            clientSeq: 3,
            kind: "network_offline",
            offsetMs: 90_000,
            occurredAt: "2026-09-04T02:11:30Z",
            durationMs: null,
          },
        ],
      }),
    ),
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const { container } = render(
    <QueryClientProvider client={client}>
      <Timeline
        attemptId={ATTEMPT_ID}
        questions={[]}
        live={false}
        note="full note must not appear"
        presentation="compact"
        onViewPaper={() => {}}
      />
    </QueryClientProvider>,
  );
  const region = await screen.findByRole("region", { name: "Diễn biến bài làm" });
  expect(within(region).getAllByRole("listitem")).toHaveLength(3);
  expect(within(region).getByText("1:12")).toBeInTheDocument();
  expect(within(region).getByText("— đang tiếp diễn")).toBeInTheDocument();
  expect(within(region).getByText("Mất kết nối")).toBeInTheDocument();
  expect(screen.queryByRole("table")).toBeNull();
  expect(screen.queryByRole("textbox")).toBeNull();
  expect(screen.queryByText("full note must not appear")).toBeNull();
  expect(
    [...container.querySelectorAll("*")]
      .flatMap((node) => [...node.classList])
      .filter((token) => /^(bg|text|border|ring)-(danger|destructive)/.test(token)),
  ).toEqual([]);
});
