import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { http } from "msw";
import type { SaveImportReview } from "@/features/imports/api";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { BASE, deferred, errorBody } from "./fixtures";
import {
  baseline,
  renderReview,
  savedFrom,
  serveReview,
  type ReviewServer,
} from "./reviewHarness";
import "@/lib/i18n";

let state: ReviewServer;

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  state = { puts: [], commits: [] };
  serveReview(baseline(), state);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the review's autosave", () => {
  it("never has two saves in flight and sends the revision the previous save returned", async () => {
    const first = deferred<void>();
    server.use(
      http.put(`${BASE}/admin/imports/:id/review`, async ({ request }) => {
        const body = (await request.json()) as SaveImportReview;
        state.puts.push(body);
        if (state.puts.length === 1) await first.promise;
        return contractJson(
          "/admin/imports/{id}/review",
          "put",
          200,
          savedFrom(baseline(), body),
        );
      }),
    );
    const { user } = await renderReview();
    const title = screen.getByLabelText("Tên đề");

    await user.type(title, "A");
    expect(state.puts, "typing stays local inside the debounce").toHaveLength(0);
    await vi.advanceTimersByTimeAsync(1500);
    await waitFor(() => expect(state.puts).toHaveLength(1));
    expect(await screen.findByText("Đang lưu…")).toBeInTheDocument();

    await user.type(title, "B");
    await vi.advanceTimersByTimeAsync(1500);
    await vi.advanceTimersByTimeAsync(3000);
    expect(state.puts, "no second PUT while the first is in flight").toHaveLength(1);

    first.resolve();
    await waitFor(() => expect(state.puts).toHaveLength(2));
    expect(state.puts[0]!.expectedRevision).toBe(1);
    expect(state.puts[1]!.expectedRevision).toBe(2);
    expect(state.puts[1]!.title).toBe("Đề thi học kỳ 1AB");
    expect(await screen.findByText(/Đã lưu \d\d:\d\d/)).toBeInTheDocument();
  });

  it("stops on a stale write, says so, and offers a reload instead of overwriting", async () => {
    server.use(
      http.put(`${BASE}/admin/imports/:id/review`, async ({ request }) => {
        state.puts.push((await request.json()) as SaveImportReview);
        return contractJson(
          "/admin/imports/{id}/review",
          "put",
          409,
          errorBody("STALE_WRITE", "Bản nhập đã được lưu ở nơi khác."),
        );
      }),
    );
    const { user } = await renderReview();
    const title = screen.getByLabelText("Tên đề");

    await user.type(title, "A");
    await vi.advanceTimersByTimeAsync(1500);

    expect(
      await screen.findByText(
        /^Bản nhập đã thay đổi ở nơi khác nên thay đổi ở đây không lưu được nữa/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tải lại" })).toBeInTheDocument();
    expect(title, "editing stops so nothing overwrites the other tab").toBeDisabled();
    await vi.advanceTimersByTimeAsync(5000);
    expect(state.puts).toHaveLength(1);
  });

  it("asks before a reload discards the edit a stale write left unsaved", async () => {
    server.use(
      http.put(`${BASE}/admin/imports/:id/review`, async ({ request }) => {
        state.puts.push((await request.json()) as SaveImportReview);
        return contractJson(
          "/admin/imports/{id}/review",
          "put",
          409,
          errorBody("STALE_WRITE", "Bản nhập đã được lưu ở nơi khác."),
        );
      }),
    );
    const { user } = await renderReview();
    await user.type(screen.getByLabelText("Tên đề"), "A");
    await vi.advanceTimersByTimeAsync(1500);
    const reload = await screen.findByRole("button", { name: "Tải lại" });

    await user.click(reload);
    const dialog = await screen.findByRole("dialog", { name: "Tải lại bản rà soát?" });
    expect(within(dialog).getByText(/sẽ bị bỏ/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Huỷ" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(reload).toHaveFocus();
    expect(screen.getByLabelText("Tên đề")).toHaveValue("Đề thi học kỳ 1A");

    await user.click(reload);
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Tải lại",
      }),
    );
    await waitFor(() =>
      expect(screen.getByLabelText("Tên đề")).toHaveValue("Đề thi học kỳ 1"),
    );
    expect(screen.getByLabelText("Tên đề")).toBeEnabled();
  });

  it("shows a failed save with a retry that resends the latest edit", async () => {
    let fail = true;
    server.use(
      http.put(`${BASE}/admin/imports/:id/review`, async ({ request }) => {
        const body = (await request.json()) as SaveImportReview;
        state.puts.push(body);
        if (fail) {
          fail = false;
          return new Response(null, { status: 503 });
        }
        return contractJson(
          "/admin/imports/{id}/review",
          "put",
          200,
          savedFrom(baseline(), body),
        );
      }),
    );
    const { user } = await renderReview();
    await user.type(screen.getByLabelText("Tên đề"), "A");
    await vi.advanceTimersByTimeAsync(1500);

    expect(await screen.findByText("Chưa lưu được")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(state.puts).toHaveLength(2));
    expect(state.puts[1]!.title).toBe("Đề thi học kỳ 1A");
    expect(await screen.findByText(/Đã lưu \d\d:\d\d/)).toBeInTheDocument();
  });
});
