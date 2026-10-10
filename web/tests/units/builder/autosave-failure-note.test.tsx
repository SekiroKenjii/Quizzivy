import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  AutosaveFailureNote,
  AutosaveStatusLabel,
} from "@/features/tests/components/AutosaveStatusLabel";
import type { AutosaveStatus } from "@/features/tests/useAutosave";
import "@/lib/i18n";

function renderBar(status: AutosaveStatus, onRetry = vi.fn()) {
  render(
    <>
      <AutosaveStatusLabel status={status} deck />
      <AutosaveFailureNote status={status} onRetry={onRetry} />
    </>,
  );
  return { user: userEvent.setup(), onRetry };
}

describe("a failed save in the builder", () => {
  it("puts a silent badge in the bar and one alert with the reason and Retry under it", async () => {
    const { user, onRetry } = renderBar({
      kind: "failed",
      message: "Dữ liệu câu hỏi không hợp lệ.",
    });

    expect(screen.getByText("Chưa lưu được")).not.toHaveAttribute("role");
    const alerts = screen.getAllByRole("alert");
    expect(alerts).toHaveLength(1);
    expect(alerts[0]).toHaveTextContent("Chưa lưu được: Dữ liệu câu hỏi không hợp lệ.");
    await user.click(within(alerts[0]!).getByRole("button", { name: "Thử lại" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("says the edit is still on screen when the failure gives no reason", () => {
    renderBar({ kind: "failed", message: "" });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Chỉnh sửa vẫn còn trên màn hình. Hãy thử lưu lại.",
    );
  });

  it("offers no Retry for a value refused before sending, which a retry would refuse again", () => {
    renderBar({ kind: "failed", message: "Còn lựa chọn để trống.", refused: true });

    expect(screen.getByRole("alert")).toHaveTextContent("Còn lựa chọn để trống.");
    expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
  });

  it("draws nothing under the bar while saves go through", () => {
    renderBar({ kind: "saving" });

    expect(screen.queryByRole("alert")).toBeNull();
  });
});
