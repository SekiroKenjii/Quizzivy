import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/AttemptReviewPage";
import { Toaster } from "@/components/ui/sonner";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { ATTEMPT_ID, BASE, review } from "./fixtures";

let flags: unknown[] = [];
let notes: unknown[] = [];

beforeEach(() => {
  flags = [];
  notes = [];
  const paper = review();
  server.use(
    http.get(`${BASE}/admin/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/admin/attempts/{id}", "get", 200, {
        ...paper,
        teacherNote: "đã hỏi Minh",
      }),
    ),
    http.get(`${BASE}/admin/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/admin/attempts/{id}/events", "get", 200, {
        startedAt: paper.attempt.startedAt,
        events: [],
        summary: paper.integrity,
      }),
    ),
    http.post(`${BASE}/admin/attempts/${ATTEMPT_ID}/flag`, async ({ request }) => {
      const body = (await request.json()) as { flagged: boolean };
      flags.push(body);
      return contractJson("/admin/attempts/{id}/flag", "post", 200, {
        ...paper.attempt,
        integrity: { ...paper.attempt.integrity, flagged: body.flagged },
      });
    }),
    http.patch(`${BASE}/admin/attempts/${ATTEMPT_ID}/note`, async ({ request }) => {
      const body = (await request.json()) as { note: string | null };
      notes.push(body);
      return contractJson("/admin/attempts/{id}/note", "patch", 200, body);
    }),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

function renderReview() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/attempts/:id", element: <AttemptReviewPage /> }],
    { initialEntries: [`/admin/attempts/${ATTEMPT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
}

describe("G-05's mark and note", () => {
  it("flags by hand with neutral words, and says so", async () => {
    renderReview();
    const user = userEvent.setup();

    await user.click(await screen.findByRole("button", { name: "Đánh dấu" }));

    await waitFor(() => expect(flags).toEqual([{ flagged: true }]));
    expect(await screen.findByText("Đã đánh dấu để xem lại")).toBeInTheDocument();
  });

  it("keeps the private note beside the timeline and autosaves it", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderReview();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });

    await user.click(await screen.findByRole("tab", { name: "Tính toàn vẹn" }));
    const note = await screen.findByRole("textbox", { name: "Ghi chú của bạn" });
    expect(note).toHaveValue("đã hỏi Minh");
    expect(
      screen.getByText("Chỉ bạn đọc được. Không hiện cho học viên."),
    ).toBeInTheDocument();

    await user.type(note, ", em nói mất điện");
    vi.advanceTimersByTime(2_000);

    await waitFor(() =>
      expect(notes).toEqual([{ note: "đã hỏi Minh, em nói mất điện" }]),
    );
  });
});
