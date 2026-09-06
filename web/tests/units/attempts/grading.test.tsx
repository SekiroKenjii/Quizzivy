import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/AttemptReviewPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { ATTEMPT_ID, BASE, ESSAY_ID, review } from "./fixtures";

let graded: unknown[] = [];
let essayScore: number | null = null;

function serve() {
  server.use(
    http.get(`${BASE}/admin/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/admin/attempts/{id}", "get", 200, review({ essayScore })),
    ),
    http.post(`${BASE}/admin/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      const body = (await request.json()) as { items: { points: number }[] };
      graded.push(body);
      essayScore = body.items[0]?.points ?? null;
      return contractJson("/admin/attempts/{id}/grade", "post", 200, {
        earned: 5 + (essayScore ?? 0),
        total: 10,
        pendingManual: 0,
      });
    }),
    http.get(`${BASE}/admin/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/admin/attempts/{id}/events", "get", 200, {
        startedAt: "2026-09-04T02:10:00Z",
        events: [],
        summary: review().integrity,
      }),
    ),
  );
}

function renderReview() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/attempts/:id", element: <AttemptReviewPage /> }],
    { initialEntries: [`/admin/attempts/${ATTEMPT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("attempt review and grading", () => {
  beforeEach(() => {
    graded = [];
    essayScore = null;
    serve();
  });

  it("opens on the essay, shows the sample answer there and for no other type", async () => {
    const user = renderReview();
    expect(
      await screen.findByText("Đáp án mẫu — chỉ bạn nhìn thấy"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("I get up at half past six every morning."),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^Câu 1,/ }));
    expect(screen.getByText("Thủ đô của Việt Nam?")).toBeInTheDocument();
    expect(
      screen.queryByText("Đáp án mẫu — chỉ bạn nhìn thấy"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("học viên chọn · đúng")).toBeInTheDocument();
  });

  it("blocks finishing while a manual item remains, and frees it once graded", async () => {
    const user = renderReview();
    const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
    expect(finish).toBeDisabled();
    expect(screen.getByText("còn 1 câu chờ chấm")).toBeInTheDocument();

    await user.clear(screen.getByLabelText("Điểm"));
    await user.type(screen.getByLabelText("Điểm"), "4");
    await user.type(screen.getByLabelText(/^Nhận xét/), "Ý tốt.");
    await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));

    await waitFor(() =>
      expect(graded).toEqual([
        { items: [{ questionId: ESSAY_ID, points: 4, comment: "Ý tốt." }] },
      ]),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Hoàn tất chấm" })).toBeEnabled(),
    );
    expect(screen.queryByText("còn 1 câu chờ chấm")).not.toBeInTheDocument();
  });

  it("refuses a mark above the ceiling before asking the server", async () => {
    const user = renderReview();
    const points = await screen.findByLabelText("Điểm");
    await user.clear(points);
    await user.type(points, "7");
    expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeDisabled();
    expect(
      within(
        screen.getByRole("button", { name: "Lưu & câu tiếp theo" }).closest("form") ??
          document.body,
      ).getByLabelText("Điểm"),
    ).toHaveAttribute("aria-invalid", "true");
  });
});
