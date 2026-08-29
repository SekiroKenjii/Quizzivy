import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import TestBuilderPage from "@/features/tests/pages/TestBuilderPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const TEST_ID = "018f0000-0000-7000-8000-0000000000a1";
const QUESTION_ID = "018f0000-0000-7000-8000-0000000000b1";

/**
 * The §8 version guard, enforced by the mock exactly as the server enforces it:
 * `updated_at` advances on every write (the tests_set_updated_at trigger fires
 * even for an outline-only save), and a PATCH whose expectedUpdatedAt does not
 * match the current one is a 409 STALE_WRITE.
 *
 * The builder used to send the value it read at mount for the whole session, so
 * the SECOND separated edit bricked it: "Bài này đang được mở ở nơi khác" in a
 * single tab, with every later edit dropped. E2E 1a missed it because it types
 * fast enough that every edit coalesces into one save.
 */
let currentVersion = "2026-01-02T00:00:00.000000Z";
let staleWrites = 0;
let accepted = 0;

function testBody() {
  return {
    id: TEST_ID,
    title: "Unit 5",
    description: null,
    status: "draft" as const,
    currentVersion: 0,
    totalPoints: 1,
    questionCount: 1,
    sections: [
      {
        id: "018f0000-0000-7000-8000-0000000000c1",
        ordinal: 0,
        title: "Ngữ pháp",
        instructions: null,
        questionIds: [QUESTION_ID],
      },
    ],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: currentVersion,
  };
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  currentVersion = "2026-01-02T00:00:00.000000Z";
  staleWrites = 0;
  accepted = 0;

  server.use(
    http.get(`${BASE}/admin/tests/:id`, () =>
      contractJson("/admin/tests/{id}", "get", 200, testBody()),
    ),
    http.get(`${BASE}/admin/questions/:id`, () =>
      contractJson("/admin/questions/{id}", "get", 200, {
        id: QUESTION_ID,
        type: "short_answer" as const,
        prompt: "Câu hỏi",
        media: null,
        audio: null,
        transcript: null,
        options: [],
        blanks: [],
        points: 1,
        explanation: null,
        sampleAnswer: null,
        tags: [],
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      }),
    ),
    http.patch(`${BASE}/admin/tests/:id`, async ({ request }) => {
      const body = (await request.json()) as { expectedUpdatedAt: string };
      if (body.expectedUpdatedAt !== currentVersion) {
        staleWrites += 1;
        return Response.json(
          { error: { code: "STALE_WRITE", message: "Đã có thay đổi ở nơi khác." } },
          { status: 409 },
        );
      }
      accepted += 1;
      // Every write advances the version, outline-only included.
      currentVersion = `2026-01-02T00:00:0${accepted}.000000Z`;
      return contractJson("/admin/tests/{id}", "patch", 200, testBody());
    }),
  );
});

afterEach(() => {
  vi.useRealTimers();
});

async function renderBuilder() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/tests/:id/edit", element: <TestBuilderPage /> }],
    { initialEntries: [`/admin/tests/${TEST_ID}/edit`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return {
    user: userEvent.setup({ advanceTimers: vi.advanceTimersByTime }),
    title: await screen.findByLabelText("Tên đề thi"),
  };
}

describe("the builder across several separated saves", () => {
  it("carries the version forward instead of resending the one it mounted with", async () => {
    const { user, title } = await renderBuilder();

    await user.type(title, "a");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(accepted).toBe(1));

    // A minute later, in teacher terms: a second edit, well outside the
    // debounce window that hid this from the E2E.
    await user.type(title, "b");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(accepted).toBe(2));

    await user.type(title, "c");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(accepted).toBe(3));

    expect(staleWrites, "no save may be refused as stale in a single tab").toBe(0);
  });

  it("stays usable: no stale banner, and publish stays available", async () => {
    const { user, title } = await renderBuilder();

    await user.type(title, "a");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(accepted).toBe(1));

    await user.type(title, "b");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(accepted).toBe(2));

    expect(screen.queryByText(/mở ở nơi khác/)).toBeNull();
    expect(screen.getByRole("button", { name: "Phát hành" })).toBeEnabled();
  });

  it("still reports a genuine conflict from another tab", async () => {
    const { user, title } = await renderBuilder();

    await user.type(title, "a");
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(accepted).toBe(1));

    // Someone else saves, moving the version out from under this builder.
    currentVersion = "2026-01-03T00:00:00.000000Z";

    await user.type(title, "b");
    await vi.advanceTimersByTimeAsync(1500);

    expect(await screen.findByText(/mở ở nơi khác/)).toBeInTheDocument();
    expect(staleWrites).toBe(1);
  });
});
