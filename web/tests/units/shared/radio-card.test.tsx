import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RadioCards, type RadioCardOption } from "@/components/shared/form/RadioCard";
import "@/lib/i18n";

const MODES: readonly RadioCardOption[] = [
  {
    value: "auto",
    label: "Tự nhận dạng",
    hint: "Phù hợp với hầu hết đề tự do. Không cần mẫu.",
  },
  {
    value: "profile",
    label: "Dùng hồ sơ đã lưu",
    hint: "Cho tệp có định dạng bạn đã nhập trước đây.",
  },
  { value: "manual", label: "Tự đánh dấu", disabled: true },
];

function Modes({
  onChange = () => {},
  className,
}: Readonly<{ onChange?: (value: string) => void; className?: string }>) {
  const [mode, setMode] = useState("auto");
  return (
    <>
      <RadioCards
        label="Cách đọc nội dung"
        value={mode}
        options={MODES}
        className={className}
        onChange={(next) => {
          onChange(next);
          setMode(next);
        }}
      />
      <button type="button">Tiếp tục</button>
    </>
  );
}

const group = () => screen.getByRole("radiogroup", { name: "Cách đọc nội dung" });
const card = (name: RegExp) => within(group()).getByRole("radio", { name });

describe("the radio cards", () => {
  it("are a radio group with its name and one radio per option", () => {
    render(<Modes />);
    expect(within(group()).getAllByRole("radio")).toHaveLength(3);
  });

  it("check the chosen card and no other", () => {
    render(<Modes />);
    expect(card(/Tự nhận dạng/)).toBeChecked();
    expect(card(/Dùng hồ sơ đã lưu/)).not.toBeChecked();
    expect(card(/Tự đánh dấu/)).not.toBeChecked();
  });

  it("draw the label and the hint inside the card", () => {
    render(<Modes />);
    const auto = card(/Tự nhận dạng/);
    expect(within(auto).getByText("Tự nhận dạng")).toHaveClass(
      "text-ui",
      "font-medium",
    );
    expect(
      within(auto).getByText("Phù hợp với hầu hết đề tự do. Không cần mẫu."),
    ).toHaveClass("text-meta", "text-muted-fg");
    expect(card(/Tự đánh dấu/).textContent).toBe("Tự đánh dấu");
  });

  it("choose a card on a click", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Modes onChange={onChange} />);
    await user.click(card(/Dùng hồ sơ đã lưu/));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith("profile");
    expect(card(/Dùng hồ sơ đã lưu/)).toBeChecked();
    expect(card(/Tự nhận dạng/)).not.toBeChecked();
  });

  it("choose the next card with an arrow key, passing a switched-off one", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Modes onChange={onChange} />);
    await user.tab();
    expect(card(/Tự nhận dạng/)).toHaveFocus();
    await user.keyboard("{ArrowDown>}");
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith("profile"));
    await user.keyboard("{/ArrowDown}");
    expect(card(/Dùng hồ sơ đã lưu/)).toHaveFocus();
    expect(card(/Dùng hồ sơ đã lưu/)).toBeChecked();
    await user.keyboard("{ArrowDown>}");
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith("auto"));
    await user.keyboard("{/ArrowDown}");
    expect(onChange).toHaveBeenCalledTimes(2);
  });

  it("are one tab stop", async () => {
    const user = userEvent.setup();
    render(<Modes />);
    await user.tab();
    expect(card(/Tự nhận dạng/)).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Tiếp tục" })).toHaveFocus();
  });

  it("cannot choose a switched-off card", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Modes onChange={onChange} />);
    expect(card(/Tự đánh dấu/)).toBeDisabled();
    await user.click(card(/Tự đánh dấu/));
    expect(onChange).not.toHaveBeenCalled();
    expect(card(/Tự nhận dạng/)).toBeChecked();
  });

  it("sit in the deck's grid of columns at least 240px wide, 10px apart", () => {
    render(<Modes />);
    expect(group()).toHaveClass(
      "grid",
      "grid-cols-[repeat(auto-fit,minmax(min(100%,240px),1fr))]",
      "gap-2.5",
    );
  });

  it("take another grid from the caller", () => {
    render(<Modes className="grid-cols-1" />);
    expect(group()).toHaveClass("grid", "grid-cols-1", "gap-2.5");
    expect(group()).not.toHaveClass(
      "grid-cols-[repeat(auto-fit,minmax(min(100%,240px),1fr))]",
    );
  });

  it("draw the card's border and its circle in the primary colour when chosen", () => {
    render(<Modes />);
    const auto = card(/Tự nhận dạng/);
    expect(auto).toHaveClass(
      "flex",
      "items-start",
      "gap-3",
      "rounded-lg",
      "border-[1.5px]",
      "border-border",
      "data-[state=checked]:border-primary",
      "bg-card",
      "px-3.5",
      "py-3",
      "text-left",
    );
    expect(auto).toHaveAttribute("data-state", "checked");
    expect(card(/Dùng hồ sơ đã lưu/)).toHaveAttribute("data-state", "unchecked");
    const circle = auto.firstElementChild!;
    expect(circle).toHaveAttribute("aria-hidden", "true");
    expect(circle).toHaveClass(
      "size-4.5",
      "rounded-full",
      "border-2",
      "border-ring",
      "group-data-[state=checked]:border-primary",
      "mt-px",
    );
    expect(circle.firstElementChild).toHaveClass(
      "size-2",
      "rounded-full",
      "group-data-[state=checked]:bg-primary",
    );
  });
});
