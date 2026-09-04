import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { ApiError } from "@/lib/api/errors";
import "@/lib/i18n";

describe("F-08's list states", () => {
  it("loads as rows, not a spinner", () => {
    render(<ListSkeleton rows={3} />);
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.queryByText("Đang tải…")).toBeNull();
  });

  it("is empty in one sentence with one action", () => {
    render(
      <EmptyState action={<button type="button">Tạo</button>}>Chưa có gì.</EmptyState>,
    );
    expect(screen.getByText("Chưa có gì.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tạo" })).toBeInTheDocument();
  });

  it("fails with a retry and the full request id", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    const error = new ApiError({
      status: 500,
      code: "UNKNOWN",
      message: "boom",
      requestId: "req_7f3a9d41",
    });
    render(
      <LoadError error={error} onRetry={onRetry}>
        Không tải được danh sách đề thi.
      </LoadError>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Không tải được danh sách đề thi.",
    );
    expect(screen.getByText("req_7f3a9d41")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Thử lại" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("hides the request id row when there is none to search for", () => {
    render(
      <LoadError error={new Error("offline")} onRetry={() => {}}>
        Không tải được.
      </LoadError>,
    );
    expect(screen.queryByText("Mã lỗi")).toBeNull();
  });
});
