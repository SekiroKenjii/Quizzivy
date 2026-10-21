import { describe, expect, it, vi } from "vitest";
import { useRef, useState } from "react";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DirtyBar } from "@/components/shared/DirtyBar";
import "@/lib/i18n";

const SENTENCE = "Bạn có thay đổi chưa lưu.";
const discard = () => screen.getByRole("button", { name: "Bỏ thay đổi" });
const save = (name = "Lưu thay đổi") => screen.getByRole("button", { name });

function inForm(
  props: Partial<{
    dirty: boolean;
    saving: boolean;
    error: string | null;
    blocked: string | null;
    errorAction: { label: string; onAction: () => void };
    onDiscard: () => void;
    onSave: () => void;
  }> = {},
) {
  const onSubmit = vi.fn((event: { preventDefault: () => void }) =>
    event.preventDefault(),
  );
  const view = render(
    <form aria-label="Hồ sơ" onSubmit={onSubmit}>
      <DirtyBar dirty onDiscard={() => {}} returnFocus={() => null} {...props} />
    </form>,
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

  it("switches both buttons off and reads Saving while it saves, keeping them focusable", async () => {
    const user = userEvent.setup();
    const onDiscard = vi.fn();
    const onSave = vi.fn();
    const { onSubmit } = inForm({ saving: true, onDiscard, onSave });
    expect(screen.queryByRole("button", { name: "Lưu thay đổi" })).toBeNull();
    expect(save("Đang lưu…")).toHaveAttribute("aria-disabled", "true");
    expect(discard()).toHaveAttribute("aria-disabled", "true");
    for (const button of [save("Đang lưu…"), discard()])
      expect(button.className).not.toMatch(/aria-disabled:opacity-|(^|\s)opacity-/);
    expect(save("Đang lưu…")).toHaveClass("aria-disabled:bg-primary/55");
    expect(save("Đang lưu…")).toBeEnabled();
    expect(discard()).toBeEnabled();
    await user.click(save("Đang lưu…"));
    expect(save("Đang lưu…")).toHaveFocus();
    await user.click(discard());
    expect(discard()).toHaveFocus();
    expect(onSave).not.toHaveBeenCalled();
    expect(onDiscard).not.toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("does not submit the form it sits in while it saves", async () => {
    const user = userEvent.setup();
    const { onSubmit } = inForm({ saving: true });
    await user.click(save("Đang lưu…"));
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

  it("reads the reason in the sentence's place, in the danger tone, and describes Save changes with it", () => {
    inForm({ blocked: "Phải bật ít nhất một cách đăng nhập." });
    const reason = screen.getByText("Phải bật ít nhất một cách đăng nhập.");
    expect(reason).toHaveClass("text-danger-ink", "text-sm", "flex-1");
    expect(screen.queryByText(SENTENCE)).toBeNull();
    expect(save()).toHaveAccessibleDescription("Phải bật ít nhất một cách đăng nhập.");
    expect(discard()).not.toHaveAttribute("aria-disabled");
  });

  it("pauses only Save changes while a reason blocks it, and ignores a press on it", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const { onSubmit } = inForm({ blocked: "Không lưu được.", onSave });
    expect(save()).toHaveAttribute("aria-disabled", "true");
    expect(save()).toBeEnabled();
    expect(save().className).not.toMatch(/aria-disabled:opacity-|(^|\s)opacity-/);
    await user.click(save());
    expect(save()).toHaveFocus();
    expect(onSave).not.toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("treats an empty reason as none", () => {
    inForm({ blocked: "" });
    expect(screen.getByText(SENTENCE)).toBeInTheDocument();
    expect(save()).not.toHaveAttribute("aria-disabled");
  });

  it("lets the error stand in for the reason, and leaves Save changes paused", () => {
    inForm({ error: "Lỗi máy chủ.", blocked: "Không lưu được." });
    expect(screen.getByRole("alert")).toHaveTextContent("Lỗi máy chủ.");
    expect(screen.queryByText("Không lưu được.")).toBeNull();
    expect(save()).toHaveAttribute("aria-disabled", "true");
    expect(save()).not.toHaveAttribute("aria-describedby");
  });

  it("offers the error's action as a button beside Discard, and only with an error", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const { rerender } = inForm({ errorAction: { label: "Tải lại", onAction } });
    expect(screen.queryByRole("button", { name: "Tải lại" })).toBeNull();

    rerender(
      <form aria-label="Hồ sơ">
        <DirtyBar
          dirty
          error="Đã lưu ở nơi khác."
          errorAction={{ label: "Tải lại", onAction }}
          onDiscard={() => {}}
          returnFocus={() => null}
        />
      </form>,
    );
    const action = screen.getByRole("button", { name: "Tải lại" });
    expect(action).toHaveAttribute("type", "button");
    expect(action).toHaveAttribute("data-variant", "outline");
    expect(action.nextElementSibling).toBe(discard());
    await user.click(action);
    expect(onAction).toHaveBeenCalledTimes(1);
  });

  it("pauses the error's action while it saves, without fading the element", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    inForm({
      saving: true,
      error: "Đã lưu ở nơi khác.",
      errorAction: { label: "Tải lại", onAction },
    });
    const action = screen.getByRole("button", { name: "Tải lại" });
    expect(action).toHaveAttribute("aria-disabled", "true");
    expect(action.className).not.toMatch(/aria-disabled:opacity-|(^|\s)opacity-/);
    await user.click(action);
    expect(onAction).not.toHaveBeenCalled();
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
      "text-sm",
      "hover:bg-muted",
      "dark:hover:bg-muted",
    );
    expect(discard()).toHaveAttribute("data-variant", "ghost");
    expect(discard()).not.toHaveClass("px-3.5");
    expect(discard()).not.toHaveClass("text-ui");
    expect(save()).toHaveClass("h-8.5", "bg-primary", "px-3.5", "text-ui");
    expect(save()).not.toHaveClass("h-9");
  });
});

function Editor() {
  const [dirty, setDirty] = useState(false);
  const target = useRef<HTMLButtonElement>(null);
  return (
    <form
      aria-label="Hồ sơ"
      onSubmit={(event) => {
        event.preventDefault();
        setDirty(false);
      }}
    >
      <button ref={target} type="button">
        Đích
      </button>
      <input aria-label="Tên" onChange={() => setDirty(true)} />
      <DirtyBar
        dirty={dirty}
        onDiscard={() => setDirty(false)}
        returnFocus={() => target.current}
      />
    </form>
  );
}

describe("focus when the unsaved-changes bar goes", () => {
  it.each([
    ["Save changes", "Lưu thay đổi"],
    ["Discard", "Bỏ thay đổi"],
  ])("moves to the caller's target after %s", async (_, name) => {
    const user = userEvent.setup();
    render(<Editor />);
    await user.type(screen.getByRole("textbox", { name: "Tên" }), "a");
    await user.click(screen.getByRole("button", { name }));
    expect(screen.queryByRole("button", { name })).toBeNull();
    expect(screen.getByRole("button", { name: "Đích" })).toHaveFocus();
  });

  it("leaves focus alone when the changes end from outside the bar", async () => {
    const user = userEvent.setup();
    render(<Editor />);
    const field = screen.getByRole("textbox", { name: "Tên" });
    await user.type(field, "a{Enter}");
    expect(screen.queryByText(SENTENCE)).toBeNull();
    expect(field).toHaveFocus();
  });

  it("forgets a failed save, so a later reset from elsewhere does not move focus", async () => {
    const user = userEvent.setup();
    const target = document.createElement("button");
    document.body.append(target);
    const bar = (dirty: boolean, saving: boolean) => (
      <form aria-label="Hồ sơ" onSubmit={(event) => event.preventDefault()}>
        <DirtyBar
          dirty={dirty}
          saving={saving}
          onDiscard={() => {}}
          returnFocus={() => target}
        />
      </form>
    );
    const view = render(bar(true, false));

    await user.click(save());
    view.rerender(bar(true, true));
    view.rerender(bar(true, false));
    act(() => (document.activeElement as HTMLElement).blur());
    view.rerender(bar(false, false));

    expect(target).not.toHaveFocus();
    target.remove();
  });
});
