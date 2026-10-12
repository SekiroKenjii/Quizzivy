import { describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CopyButton } from "@/components/shared/CopyButton";
import { CopyField } from "@/components/shared/CopyField";
import "@/lib/i18n";

const LINK = "https://quizzivy.app/join/K7M2-QX9P";
const FAILED = "Không sao chép được. Hãy chọn liên kết rồi sao chép.";

const IDLE = `<button data-slot="button" data-variant="ghost" data-size="xs" class="inline-flex shrink-0 items-center justify-center whitespace-nowrap transition-all outline-none focus-visible:border-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&amp;_svg]:pointer-events-none [&amp;_svg]:shrink-0 hover:text-accent-foreground dark:hover:bg-accent/50 h-7 rounded-sm in-data-[scale=deck]:h-6.5 hover:bg-hover gap-[5px] px-2 text-xs font-medium [&amp;_svg:not([class*='size-'])]:size-[13px]" type="button"><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-copy" aria-hidden="true"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>Sao chép</button><span role="status" class="sr-only"></span>`;
const COPIED = `<button data-slot="button" data-variant="ghost" data-size="xs" class="inline-flex shrink-0 items-center justify-center whitespace-nowrap transition-all outline-none focus-visible:border-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&amp;_svg]:pointer-events-none [&amp;_svg]:shrink-0 hover:text-accent-foreground dark:hover:bg-accent/50 h-7 rounded-sm in-data-[scale=deck]:h-6.5 hover:bg-hover gap-[5px] px-2 text-xs font-medium [&amp;_svg:not([class*='size-'])]:size-[13px]" type="button"><svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-check" aria-hidden="true"><path d="M20 6 9 17l-5-5"></path></svg>Đã sao chép</button><span role="status" class="sr-only">Đã sao chép</span>`;

const field = () =>
  screen.getByRole<HTMLInputElement>("textbox", { name: "Liên kết tham gia" });

function renderField(props: Partial<{ copyLabel: string; mono: boolean }> = {}) {
  return render(
    <CopyField
      label="Liên kết tham gia"
      value={LINK}
      failedMessage={FAILED}
      {...props}
    />,
  );
}

describe("the copy field", () => {
  it("shows the value in a labelled field nobody can edit", async () => {
    const user = userEvent.setup();
    renderField();
    expect(field()).toHaveValue(LINK);
    expect(field()).toHaveAttribute("readonly");
    await user.type(field(), "x");
    expect(field()).toHaveValue(LINK);
    expect(screen.getByText("Liên kết tham gia")).toHaveClass(
      "text-meta",
      "font-medium",
    );
  });

  it("copies the value and says so for two seconds", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const writeText = vi.spyOn(navigator.clipboard, "writeText");
    renderField();
    await user.click(screen.getByRole("button", { name: "Sao chép" }));

    expect(writeText).toHaveBeenCalledTimes(1);
    expect(writeText).toHaveBeenCalledWith(LINK);
    expect(screen.getByRole("status")).toHaveTextContent("Đã sao chép");
    expect(screen.getByRole("button", { name: "Đã sao chép" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
    await act(() => vi.advanceTimersByTimeAsync(2000));
    expect(screen.getByRole("button", { name: "Sao chép" })).toBeInTheDocument();
    expect(screen.getByRole("status")).toBeEmptyDOMElement();
    vi.useRealTimers();
  });

  it("leaves the field alone after a copy that worked", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
    renderField();
    await user.click(screen.getByRole("button", { name: "Sao chép" }));
    await screen.findByRole("button", { name: "Đã sao chép" });
    expect(field()).not.toHaveFocus();
    expect(field().selectionEnd).toBe(field().selectionStart);
  });

  it("selects the whole value and says how to copy by hand when the clipboard refuses", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(new Error("denied"));
    renderField();
    await user.click(screen.getByRole("button", { name: "Sao chép" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(FAILED);
    expect(field()).toHaveFocus();
    expect(field().selectionStart).toBe(0);
    expect(field().selectionEnd).toBe(LINK.length);
    expect(screen.getByRole("status")).toBeEmptyDOMElement();
  });

  it("names the button as the caller asks", () => {
    renderField({ copyLabel: "Sao chép liên kết" });
    expect(
      screen.getByRole("button", { name: "Sao chép liên kết" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Sao chép" })).toBeNull();
  });

  it("sets a code in the monospace face only when asked", () => {
    const plain = renderField();
    expect(field()).not.toHaveClass("font-mono");
    plain.unmount();
    renderField({ mono: true });
    expect(field()).toHaveClass("font-mono");
  });

  it("draws the deck's 28px bordered action button beside the field", () => {
    renderField();
    const button = screen.getByRole("button", { name: "Sao chép" });
    expect(button).toHaveClass(
      "h-7",
      "in-data-[scale=deck]:h-7",
      "rounded-seg",
      "border",
      "border-border",
      "bg-card",
      "px-2.5",
      "text-xs",
      "font-medium",
      "hover:bg-muted",
      "dark:hover:bg-muted",
    );
    expect(button).not.toHaveClass("in-data-[scale=deck]:h-6.5");
    expect(button).not.toHaveClass("hover:bg-hover");
    expect(button).not.toHaveClass("px-2");
    expect(button.parentElement).toHaveClass("flex", "flex-wrap");
    expect(field()).toHaveClass("flex-[1_1_10rem]", "w-auto");
  });
});

describe("the copy button", () => {
  it("is today's markup, byte for byte, when it is given none of the new props", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
    const { container } = render(<CopyButton value="abc" failedMessage={FAILED} />);
    expect(container.innerHTML).toBe(IDLE);

    await user.click(screen.getByRole("button", { name: "Sao chép" }));
    await screen.findByRole("button", { name: "Đã sao chép" });
    expect(container.innerHTML).toBe(COPIED);
  });

  it("tells the caller once when the clipboard refuses", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(new Error("denied"));
    const onFailed = vi.fn();
    render(<CopyButton value="abc" failedMessage={FAILED} onFailed={onFailed} />);
    await user.click(screen.getByRole("button", { name: "Sao chép" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(FAILED);
    expect(onFailed).toHaveBeenCalledTimes(1);
  });

  it("does not tell the caller about a copy that worked", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
    const onFailed = vi.fn();
    render(<CopyButton value="abc" failedMessage={FAILED} onFailed={onFailed} />);
    await user.click(screen.getByRole("button", { name: "Sao chép" }));
    await screen.findByRole("button", { name: "Đã sao chép" });
    expect(onFailed).not.toHaveBeenCalled();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("reads the caller's label until it has copied, then Copied", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
    render(<CopyButton value="abc" failedMessage={FAILED} label="Sao chép mã" />);
    await user.click(screen.getByRole("button", { name: "Sao chép mã" }));
    expect(
      await screen.findByRole("button", { name: "Đã sao chép" }),
    ).toBeInTheDocument();
  });

  it("lets the caller's classes replace its own", () => {
    render(
      <CopyButton
        value="abc"
        failedMessage={FAILED}
        className="hover:bg-muted px-2.5"
      />,
    );
    const button = screen.getByRole("button", { name: "Sao chép" });
    expect(button).toHaveClass("px-2.5", "hover:bg-muted", "gap-[5px]", "text-xs");
    expect(button).not.toHaveClass("px-2");
    expect(button).not.toHaveClass("hover:bg-hover");
  });
});
