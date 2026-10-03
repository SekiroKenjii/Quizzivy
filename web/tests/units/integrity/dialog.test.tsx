import { afterEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrikeDialog } from "@/features/integrity/components/StrikeDialog";
import { StrikeIndicator } from "@/features/integrity/components/StrikeIndicator";
import { strikeState } from "@/features/integrity/strikes";
import type { IntegrityPolicy } from "@/features/take-test/api";
import i18n from "@/lib/i18n";

const policy: IntegrityPolicy = {
  requireFullscreen: false,
  blockCopyPaste: true,
  maxFocusLoss: 2,
  onLimitExceeded: "flag",
  minAwayMs: 3000,
};

const STAY = "Hãy ở lại trang này cho đến khi nộp bài.";
const SAFE = "Câu trả lời của bạn vẫn an toàn.";
const TOLD = "Giáo viên đã được báo.";
const BACK = "Quay lại bài làm";

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
      <StrikeDialog state={state} strikes={strikes} />
    </>
  );
}

const dialog = () => screen.getByRole("alertdialog");
const noDialog = () => expect(screen.queryByRole("alertdialog")).toBeNull();
const body = () => dialog().querySelector('[data-slot="dialog-description"]');
const tick = () => new Promise((resolve) => setTimeout(resolve, 0));

afterEach(async () => {
  await act(() => i18n.changeLanguage("vi"));
});

describe("the frame", () => {
  it("is the deck's alert: 420px, the eye-off tile, the title, one body and one full-width button", () => {
    render(<Harness strikes={1} />);

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(dialog()).toHaveAccessibleName("Bạn vừa rời trang làm bài");
    expect(dialog()).toHaveAccessibleDescription(
      `Lần này được tính là lần 1 trong 2 lần được phép. ${STAY}`,
    );
    expect(dialog()).toHaveClass(
      "w-[min(26.25rem,calc(100%-1.5rem))]",
      "bg-card",
      "shadow-float",
      "rounded-2xl",
      "p-5.5",
      "gap-3",
      "items-start",
    );

    const tile = dialog().querySelector("svg")?.parentElement;
    expect(tile).toHaveClass("size-10", "rounded-lg", "bg-warning-soft");
    expect(tile).toHaveClass("text-warning-ink");
    expect(tile).toHaveAttribute("aria-hidden", "true");
    expect(dialog().querySelector("svg")).toHaveClass("lucide-eye-off", "size-5");

    expect(
      screen.getByRole("heading", { name: "Bạn vừa rời trang làm bài" }),
    ).toHaveClass("text-lg", "font-semibold");
    expect(body()).toHaveClass("text-muted-fg", "text-base", "leading-[1.55]");
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(screen.getByRole("button", { name: BACK })).toHaveClass(
      "self-stretch",
      "h-11.5",
      "text-md",
      "bg-primary",
    );
  });
});

describe("what the dialog says", () => {
  it("counts this absence against the allowance while some of it is left", () => {
    render(<Harness strikes={1} />);
    expect(body()).toHaveTextContent(
      `Lần này được tính là lần 1 trong 2 lần được phép. ${STAY}`,
    );
  });

  it("says the same at the last one allowed, which is not yet past the allowance", () => {
    render(<Harness strikes={2} />);
    expect(body()).toHaveTextContent(
      `Lần này được tính là lần 2 trong 2 lần được phép. ${STAY}`,
    );
    expect(body()).not.toHaveTextContent("Giáo viên");
  });

  it("says the teacher has been told and the answers are safe once past it under flag", () => {
    render(<Harness strikes={1} baseline={2} />);
    expect(body()).toHaveTextContent(
      `Bạn đã rời trang làm bài 3 lần, nhiều hơn 2 lần được phép. ${TOLD} ${SAFE}`,
    );
  });

  it("never names the teacher under warn, which is the dialog and nothing else", () => {
    render(<Harness strikes={1} baseline={2} over={{ onLimitExceeded: "warn" }} />);
    expect(body()).toHaveTextContent(
      `Bạn đã rời trang làm bài 3 lần, nhiều hơn 2 lần được phép. ${SAFE} ${STAY}`,
    );
    expect(body()).not.toHaveTextContent(/giáo viên/i);
  });

  it("reads the server's count as the starting point", () => {
    render(<Harness strikes={1} baseline={1} />);
    expect(body()).toHaveTextContent(
      "Lần này được tính là lần 2 trong 2 lần được phép.",
    );
  });

  it("states no number when no absence is allowed", () => {
    render(<Harness strikes={1} over={{ maxFocusLoss: -1 }} />);
    expect(body()).toHaveTextContent(`Không được rời trang làm bài. ${TOLD} ${SAFE}`);
    expect(body()).not.toHaveTextContent(/\d/);
  });

  it("warns without the teacher when no absence is allowed under warn", () => {
    render(
      <Harness strikes={2} over={{ maxFocusLoss: -1, onLimitExceeded: "warn" }} />,
    );
    expect(body()).toHaveTextContent(`Không được rời trang làm bài. ${SAFE} ${STAY}`);
    expect(body()).not.toHaveTextContent(/giáo viên|\d/i);
  });

  it("says the test is submitted after the allowance under auto_submit", () => {
    render(
      <Harness
        strikes={1}
        over={{ maxFocusLoss: 3, onLimitExceeded: "auto_submit" }}
      />,
    );
    expect(body()).toHaveTextContent(
      "Lần này được tính là lần 1 trong 3 lần được phép. Sau đó, bài sẽ được nộp ngay.",
    );
  });

  it("says the next absence submits the test at the last one allowed under auto_submit", () => {
    render(<Harness strikes={2} over={{ onLimitExceeded: "auto_submit" }} />);
    expect(body()).toHaveTextContent(
      "Lần này được tính là lần 2 trong 2 lần được phép. Nếu bạn rời trang lần nữa, bài sẽ được nộp ngay.",
    );
  });

  it("does not say the next absence submits while one more is allowed under auto_submit", () => {
    render(<Harness strikes={1} over={{ onLimitExceeded: "auto_submit" }} />);
    expect(body()).toHaveTextContent(
      "Lần này được tính là lần 1 trong 2 lần được phép. Sau đó, bài sẽ được nộp ngay.",
    );
  });

  it("says the test is being submitted if it is ever shown past an auto_submit allowance", () => {
    render(
      <Harness strikes={1} baseline={2} over={{ onLimitExceeded: "auto_submit" }} />,
    );
    expect(body()).toHaveTextContent(
      "Bạn đã rời trang làm bài quá số lần được phép nên bài đang được nộp.",
    );
  });

  it("uses the deck's English, with the singular for an allowance of one", async () => {
    await act(() => i18n.changeLanguage("en"));
    const view = render(<Harness strikes={1} />);
    expect(dialog()).toHaveAccessibleName("You left the test");
    expect(body()).toHaveTextContent(
      "That counts as 1 of 2 times allowed. Stay on this page until you submit.",
    );
    expect(
      screen.getByRole("button", { name: "Back to the test" }),
    ).toBeInTheDocument();

    view.rerender(<Harness strikes={1} baseline={2} />);
    expect(body()).toHaveTextContent(
      "You have left 3 times, more than the 2 allowed. Your teacher has been told. Your answers are safe.",
    );

    view.rerender(<Harness strikes={1} over={{ maxFocusLoss: 1 }} />);
    expect(body()).toHaveTextContent("That counts as 1 of 1 time allowed.");
  });

  it.each([
    [
      { maxFocusLoss: 0 },
      1,
      "Leaving the test is recorded. Stay on this page until you submit.",
    ],
    [
      { maxFocusLoss: -1 },
      1,
      "Leaving the test is not allowed. Your teacher has been told. Your answers are safe.",
    ],
    [
      { maxFocusLoss: -1, onLimitExceeded: "warn" },
      1,
      "Leaving the test is not allowed. Your answers are safe. Stay on this page until you submit.",
    ],
    [
      { onLimitExceeded: "warn" },
      3,
      "You have left 3 times, more than the 2 allowed. Your answers are safe. Stay on this page until you submit.",
    ],
    [
      { maxFocusLoss: 3, onLimitExceeded: "auto_submit" },
      1,
      "That counts as 1 of 3 times allowed. After that, your test is submitted.",
    ],
    [
      { maxFocusLoss: 1, onLimitExceeded: "auto_submit" },
      1,
      "That counts as 1 of 1 time allowed. If you leave again, your test is submitted.",
    ],
  ] as const)("says %o after %i in English", async (over, strikes, sentence) => {
    await act(() => i18n.changeLanguage("en"));
    render(<Harness strikes={strikes} over={over} />);
    expect(body()?.textContent).toBe(sentence);
  });

  it.each(["vi", "en"])(
    "never calls an absence a violation or cheating, whatever the policy (%s)",
    async (language) => {
      await act(() => i18n.changeLanguage(language));
      const said: string[] = [];
      for (const maxFocusLoss of [0, -1, 1, 2, 3]) {
        for (const onLimitExceeded of ["warn", "flag", "auto_submit"] as const) {
          for (const strikes of [1, 2, 3, 4]) {
            const view = render(
              <Harness strikes={strikes} over={{ maxFocusLoss, onLimitExceeded }} />,
            );
            const text = body()?.textContent ?? "";
            said.push(text);
            if (onLimitExceeded === "warn")
              expect(text).not.toMatch(/giáo viên|teacher/i);
            if (maxFocusLoss <= 0) expect(text).not.toMatch(/\d/);
            view.unmount();
          }
        }
      }
      expect(said).toHaveLength(60);
      expect(said.every((text) => text.length > 0)).toBe(true);
      expect(said.join(" ")).not.toMatch(/vi phạm|gian lận|violat|cheat/i);
    },
  );
});

describe("when it opens", () => {
  it("opens again on the next absence, with the new count", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Harness strikes={1} />);
    expect(body()).toHaveTextContent("lần 1 trong 2");

    await user.click(screen.getByRole("button", { name: BACK }));
    noDialog();

    rerender(<Harness strikes={2} />);
    expect(body()).toHaveTextContent("lần 2 trong 2");
  });

  it("does not open for an absence that was not counted", () => {
    render(<Harness strikes={0} />);
    noDialog();
  });

  it("speaks once a sitting when there is no limit", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Harness strikes={1} over={{ maxFocusLoss: 0 }} />);
    expect(body()).toHaveTextContent(
      `Mỗi lần rời trang làm bài đều được ghi lại. ${STAY}`,
    );
    expect(body()).not.toHaveTextContent(/\d/);

    await user.click(screen.getByRole("button", { name: BACK }));
    rerender(<Harness strikes={2} over={{ maxFocusLoss: 0 }} />);
    noDialog();
  });
});

describe("never trapping the student", () => {
  it("puts focus on the only way out", () => {
    render(<Harness strikes={1} />);
    expect(screen.getByRole("button", { name: BACK })).toHaveFocus();
  });

  it("has no close button and does not close from the scrim", async () => {
    const user = userEvent.setup();
    render(<Harness strikes={1} />);
    expect(screen.queryByRole("button", { name: "Đóng" })).toBeNull();

    await tick();
    const scrim = document.querySelector('[data-slot="dialog-overlay"]');
    if (scrim === null) throw new Error("no scrim");
    await user.click(scrim);
    expect(dialog()).toBeInTheDocument();
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

    await user.click(screen.getByRole("button", { name: BACK }));
    await user.click(screen.getByRole("button", { name: "Nộp bài" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });
});

describe("the standing count", () => {
  const indicator = (count: number, over: Partial<IntegrityPolicy> = {}) =>
    render(<StrikeIndicator state={strikeState({ ...policy, ...over }, count)} />)
      .container;

  it("is a plain number while some of the allowance is left", () => {
    const view = indicator(1);
    expect(view).toHaveTextContent("Còn 1 lần rời trang");
    expect(view.firstElementChild).not.toHaveClass("font-medium");
  });

  it("reads in the full ink once the allowance is spent", () => {
    const view = indicator(2);
    expect(view).toHaveTextContent("Hết lần rời trang");
    expect(view.firstElementChild).toHaveClass("text-fg", "font-medium");
  });

  it("says the teacher has been told, in the warning ink, past the allowance under flag", () => {
    const view = indicator(3);
    expect(view).toHaveTextContent("Giáo viên đã được báo");
    expect(view.firstElementChild).toHaveClass("text-warning-ink", "font-medium");
  });

  it("does not name the teacher past the allowance under warn", () => {
    const view = indicator(3, { onLimitExceeded: "warn" });
    expect(view).toHaveTextContent("Quá số lần rời trang");
    expect(view).not.toHaveTextContent(/giáo viên/i);
    expect(view.firstElementChild).toHaveClass("text-warning-ink");
  });

  it("says the teacher has been told in English too", async () => {
    await act(() => i18n.changeLanguage("en"));
    expect(indicator(3)).toHaveTextContent("Your teacher has been told");
  });

  it("starts spent when no absence is allowed, and is absent when there is no limit", () => {
    expect(indicator(0, { maxFocusLoss: -1 })).toHaveTextContent("Hết lần rời trang");
    expect(indicator(5, { maxFocusLoss: 0 })).toBeEmptyDOMElement();
  });
});
