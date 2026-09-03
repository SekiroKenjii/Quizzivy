import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import QuestionEditorPage from "@/features/question-bank/pages/QuestionEditorPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const ID = "018f0000-0000-7000-8000-0000000000b1";

let fetches = 0;

function question(url: string) {
  return {
    id: ID,
    type: "short_answer" as const,
    prompt: "Người phụ nữ đề nghị làm gì?",
    points: 1,
    tags: [],
    usedInTests: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    media: {
      id: "018f0000-0000-7000-8000-0000000000e1",
      kind: "audio" as const,
      url,
      mimeType: "audio/mpeg",
      bytes: 1024,
      durationMs: 110_000,
      originalFilename: "unit5-listening-2.mp3",
      createdAt: "2026-01-01T00:00:00Z",
    },
    audio: {
      maxPlays: 2,
      allowSeek: false,
      showTranscriptAfterSubmit: true,
    },
  };
}

beforeEach(() => {
  fetches = 0;
  server.use(
    http.get(`${BASE}/admin/questions/:id`, () => {
      fetches += 1;
      return contractJson(
        "/admin/questions/{id}",
        "get",
        200,
        question(`https://example.test/a.mp3?sig=${fetches}`),
      );
    }),
  );
});

function renderEditor() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/question-bank/:id", element: <QuestionEditorPage /> }],
    { initialEntries: [`/admin/question-bank/${ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

/**
 * Issue #43: the shared player's `onError` was wired at one of its two call
 * sites, so the question editor swallowed an expired signed URL entirely — no
 * message, no reason, and no retry.
 */
describe("an expired audio URL in the question editor", () => {
  it("says so instead of a play button that does nothing", async () => {
    renderEditor();

    const player = await screen.findByLabelText("unit5-listening-2.mp3");
    fireEvent.error(player);

    expect(await screen.findByRole("alert")).toHaveTextContent(/hết hạn/);
  });

  it("recovers by refetching the question for a fresh URL", async () => {
    const user = renderEditor();

    const player = await screen.findByLabelText("unit5-listening-2.mp3");
    fireEvent.error(player);
    await screen.findByRole("alert");

    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    await waitFor(() => expect(fetches).toBe(2));
    const refreshed = await screen.findByLabelText("unit5-listening-2.mp3");
    expect(refreshed).toHaveAttribute("src", "https://example.test/a.mp3?sig=2");
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
