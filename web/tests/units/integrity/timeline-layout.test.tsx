import { afterEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { Timeline } from "@/features/integrity/components/Timeline";
import { contractJson } from "@tests/support/contractResponse";
import { server } from "@tests/support/server";
import { viewport } from "@tests/support/viewport";
import { ATTEMPT_ID, BASE, review } from "../attempts/fixtures";
import "@/lib/i18n";

afterEach(() => {
  vi.unstubAllGlobals();
});

const FIXED_COLUMNS = /^(grid-cols-[3-9]|col-span-[2-9])$/;

it("keeps the full timeline inside a phone's width, the table scrolling in a named region", async () => {
  viewport("phone");
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt: "2026-09-04T02:10:00Z",
        summary: review().integrity,
        events: [
          {
            id: 1,
            sessionId: "018f0000-0000-7000-8000-00000000ab01",
            clientSeq: 1,
            kind: "tab_hidden",
            offsetMs: 10_000,
            occurredAt: "2026-09-04T02:10:10Z",
            durationMs: 32_000,
          },
        ],
      }),
    ),
  );
  const { container } = render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <Timeline
        attemptId={ATTEMPT_ID}
        questions={[]}
        live={false}
        note={null}
        onViewPaper={() => {}}
      />
    </QueryClientProvider>,
  );
  const scroller = await screen.findByRole("region", { name: "Diễn biến" });
  expect(scroller).toHaveAttribute("tabindex", "0");
  expect(scroller).toHaveClass("overflow-x-auto");
  expect(within(scroller).getByRole("table")).toHaveClass("min-w-[34rem]");
  const unbounded = [...container.querySelectorAll("*")]
    .flatMap((node) => [...node.classList])
    .filter((token) => FIXED_COLUMNS.test(token));
  expect(unbounded).toEqual([]);
});
