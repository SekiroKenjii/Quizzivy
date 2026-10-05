import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DirtyBar } from "@/components/shared/DirtyBar";
import { DeckScale } from "@/components/ui/deck-scale";
import "@/lib/i18n";

const SENTENCE = "Bạn có thay đổi chưa lưu.";
const discard = () => screen.getByRole("button", { name: "Bỏ thay đổi" });
const save = (name = "Lưu thay đổi") => screen.getByRole("button", { name });

function inForm(
  props: Partial<{
    dirty: boolean;
    saving: boolean;
    error: string | null;
    onDiscard: () => void;
    onSave: () => void;
  }> = {},
) {
  const onSubmit = vi.fn((event: { preventDefault: () => void }) =>
    event.preventDefault(),
  );
  const view = render(
    <DeckScale>
      <form aria-label="Hồ sơ" onSubmit={onSubmit}>
        <DirtyBar dirty onDiscard={() => {}} {...props} />
      </form>
    </DeckScale>,
  );
  return { onSubmit, ...view };
}

describe("the unsaved-changes bar", () => {
  it("renders nothing while there is nothing to save", () => {
    inForm({ dirty: false });
    expect(screen.getByRole("form", { name: "Hồ sơ" })).toBeEmptyDOMElement();
    expect(screen.queryByText(SENTENCE)).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("says there are unsaved changes, beside Discard and Save changes", () => {
    inForm();
    expect(screen.getByText(SENTENCE)).toHaveClass(
      "text-muted-fg",
      "text-sm",
      "flex-1",
    );
    expect(discard()).toBeEnabled();
    expect(save()).toBeEnabled();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("discards without submitting the form it sits in", async () => {
    const user = userEvent.setup();
    const onDiscard = vi.fn();
    const { onSubmit } = inForm({ onDiscard });
    expect(discard()).toHaveAttribute("type", "button");
    await user.click(discard());
    expect(onDiscard).toHaveBeenCalledTimes(1);
    expect(onDiscard).toHaveBeenCalledWith();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("submits the form it sits in when Save changes is pressed", async () => {
    const user = userEvent.setup();
    const { onSubmit } = inForm();
    expect(save()).toHaveAttribute("type", "submit");
    await user.click(save());
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("calls the caller's save instead, and submits nothing, when it is given one", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const { onSubmit } = inForm({ onSave });
    expect(save()).toHaveAttribute("type", "button");
    await user.click(save());
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("switches both buttons off and reads Saving while it saves", async () => {
    const user = userEvent.setup();
    const onDiscard = vi.fn();
    const onSave = vi.fn();
    const { onSubmit } = inForm({ saving: true, onDiscard, onSave });
    expect(screen.queryByRole("button", { name: "Lưu thay đổi" })).toBeNull();
    expect(save("Đang lưu…")).toBeDisabled();
    expect(discard()).toBeDisabled();
    await user.click(save("Đang lưu…"));
    await user.click(discard());
    expect(onSave).not.toHaveBeenCalled();
    expect(onDiscard).not.toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("puts the error in the sentence's place, as an alert", () => {
    inForm({ error: "Không lưu được. Hãy thử lại." });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Không lưu được. Hãy thử lại.");
    expect(alert).toHaveClass("text-danger-ink", "text-sm", "flex-1");
    expect(screen.queryByText(SENTENCE)).toBeNull();
    expect(save()).toBeEnabled();
  });

  it("keeps the sentence when the error is null", () => {
    inForm({ error: null });
    expect(screen.getByText(SENTENCE)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("sticks 12px above the bottom as the deck's floating card, and wraps", () => {
    inForm();
    const bar = document.querySelector('[data-slot="dirty-bar"]')!;
    expect(bar).toHaveClass(
      "sticky",
      "bottom-3",
      "flex",
      "flex-wrap",
      "items-center",
      "gap-2.5",
      "rounded-xl",
      "border",
      "bg-card",
      "shadow-float",
      "py-2.5",
      "pr-3",
      "pl-4",
    );
    expect(bar.className).not.toMatch(/(^|\s)w-/);
  });

  it("draws both buttons 34px high, Discard flat at 13px and 12px of padding", () => {
    inForm();
    expect(discard()).toHaveClass(
      "h-8.5",
      "px-3",
      "in-data-[scale=deck]:px-3",
      "in-data-[scale=deck]:text-sm",
      "hover:bg-muted",
      "dark:hover:bg-muted",
    );
    expect(discard()).toHaveAttribute("data-variant", "ghost");
    expect(discard()).not.toHaveClass("in-data-[scale=deck]:px-3.5");
    expect(discard()).not.toHaveClass("in-data-[scale=deck]:text-ui");
    expect(save()).toHaveClass(
      "h-8.5",
      "bg-primary",
      "in-data-[scale=deck]:px-3.5",
      "in-data-[scale=deck]:text-ui",
    );
    expect(save()).not.toHaveClass("h-9");
  });
});
