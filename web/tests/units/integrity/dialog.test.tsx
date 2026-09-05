import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrikeDialog } from "@/features/integrity/components/StrikeDialog";
import { strikeState } from "@/features/integrity/strikes";
import type { IntegrityPolicy } from "@/features/take-test/api";
import "@/lib/i18n";

const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 2,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

/**
 * Mirrors the page: the count is the server's baseline plus this sitting's
 * strikes, and a submit control sits underneath the dialog the whole time.
 */
function Harness({
  strikes,
  baseline = 0,
  over = {},
  onSubmit = () => {},
}: Readonly<{
  strikes: number;
  baseline?: number;
  over?: Partial<IntegrityPolicy>;
  onSubmit?: () => void;
}>) {
  const state = strikeState({ ...policy, ...over }, baseline + strikes);
  return (
    <>
      <button type="button" onClick={onSubmit}>
        Nộp bài
      </button>
      <StrikeDialog state={state} strikes={strikes} lastAwayMs={24_000} />
    </>
  );
}

const dialog = () => screen.getByRole("dialog");
const noDialog = () => expect(screen.queryByRole("dialog")).toBeNull();
const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("what the dialog says", () => {
  it("states what happened, what is left, and that the clock runs", () => {
    render(<Harness strikes={1} />);
    expect(dialog()).toHaveTextContent("Bạn vừa rời khỏi trang làm bài");
    expect(dialog()).toHaveTextContent("chuyển sang cửa sổ khác trong 24 giây");
    expect(dialog()).toHaveTextContent(
      "Bạn còn 1 lần nữa trước khi bài được đánh dấu để giáo viên xem lại",
    );
    expect(dialog()).toHaveTextContent("Đồng hồ vẫn đang chạy trong lúc này.");
  });

  it("says what the next episode does once the allowance is spent", () => {
    render(<Harness strikes={2} />);
    expect(dialog()).toHaveTextContent("Bạn đã dùng hết số lần rời trang cho phép.");
    expect(dialog()).toHaveTextContent(
      "Nếu rời trang thêm lần nữa, bài sẽ được đánh dấu",
    );
  });

  // §10.2's `flag`: attempt marked for the admin, student told.
  it("tells the student the attempt is marked once the limit is exceeded", () => {
    render(<Harness strikes={1} baseline={2} />);
    expect(dialog()).toHaveTextContent("nên bài được đánh dấu để giáo viên xem lại");
    expect(dialog()).toHaveTextContent("điểm không bị trừ tự động");
    expect(dialog()).toHaveTextContent("chỉ ghi nhận việc trang này mất tập trung");
  });

  it("never says marked under warn, which is dialog only", () => {
    render(<Harness strikes={1} baseline={2} over={{ onLimitExceeded: "warn" }} />);
    expect(dialog()).toHaveTextContent("Giáo viên sẽ thấy các lần rời trang");
    expect(dialog()).not.toHaveTextContent("đánh dấu");
  });

  it("reads the server's count as the starting point", () => {
    render(<Harness strikes={1} baseline={1} />);
    expect(dialog()).toHaveTextContent("Bạn đã dùng hết số lần rời trang cho phép.");
  });
});

describe("when it opens", () => {
  it("opens again on the next episode, with the new count", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Harness strikes={1} />);
    expect(dialog()).toHaveTextContent("còn 1 lần");

    await user.click(screen.getByRole("button", { name: "Tiếp tục làm bài" }));
    noDialog();

    rerender(<Harness strikes={2} />);
    expect(dialog()).toHaveTextContent("dùng hết số lần rời trang");
  });

  it("does not open for an episode that was not counted", () => {
    render(<Harness strikes={0} />);
    noDialog();
  });

  it("speaks once a sitting when there is no limit", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Harness strikes={1} over={{ maxFocusLoss: 0 }} />);
    expect(dialog()).toHaveTextContent("Giáo viên có thể xem lại các lần rời trang");
    expect(dialog()).not.toHaveTextContent("còn");

    await user.click(screen.getByRole("button", { name: "Tiếp tục làm bài" }));
    rerender(<Harness strikes={2} over={{ maxFocusLoss: 0 }} />);
    noDialog();
  });
});

/**
 * Non-dismissible means it cannot be waved away unread -- not that the student
 * is trapped (§10.2). Escape acknowledges it like the button does, and the
 * submit control underneath is reachable the moment it closes.
 */
describe("never trapping the student", () => {
  it("puts focus on the only way out", () => {
    render(<Harness strikes={1} />);
    expect(screen.getByRole("button", { name: "Tiếp tục làm bài" })).toHaveFocus();
  });

  it("has no close button and does not close from the scrim", async () => {
    const user = userEvent.setup();
    render(<Harness strikes={1} />);
    expect(screen.queryByRole("button", { name: "Đóng" })).toBeNull();

    await tick();
    const scrim = document.querySelector('[data-slot="dialog-overlay"]');
    if (scrim === null) throw new Error("no scrim");
    await user.click(scrim);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("lets Escape acknowledge it rather than swallowing the key", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<Harness strikes={1} onSubmit={onSubmit} />);

    await user.keyboard("{Escape}");
    noDialog();

    await user.click(screen.getByRole("button", { name: "Nộp bài" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("leaves a submit path reachable after the button too", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(<Harness strikes={1} onSubmit={onSubmit} />);

    await user.click(screen.getByRole("button", { name: "Tiếp tục làm bài" }));
    await user.click(screen.getByRole("button", { name: "Nộp bài" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });
});
